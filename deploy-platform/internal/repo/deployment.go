package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gantry/deploy-platform/internal/model"
	"gantry/deploy-platform/internal/status"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeploymentFilter 是管理页面查询发布记录时使用的可选条件
type DeploymentFilter struct {
	AppID    int64
	Status   string
	Page     int
	PageSize int
}

// CreateDeployment 创建 pending 发布任务并阻止同一应用同时存在多个活跃任务
// fromVersionID 为 0 表示没有回滚版本，messageID 用于关联 RabbitMQ 消息
func (r *Repo) CreateDeployment(ctx context.Context, appID, versionID, fromVersionID int64, messageID string) (*model.Deployment, error) {
	dep := &model.Deployment{
		AppID:     appID,
		VersionID: versionID,
		Status:    string(status.Pending),
		MessageID: messageID,
	}
	if fromVersionID != 0 {
		dep.FromVersionID = &fromVersionID
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁住稳定存在的 App 行，让活跃检查和任务创建形成每应用串行临界区
		if err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
		}).Select("id").First(&model.App{}, appID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Deployment{}).
			Where("app_id = ? AND status IN ('pending', 'queued', 'running')", appID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			// 回调返回业务错误会回滚事务并释放 App 行锁
			return ErrActiveDeploy
		}
		// Create 成功返回 nil 后由 Transaction 提交并释放 App 行锁
		return tx.Create(dep).Error
	})
	return dep, err
}

// GetDeployment 按主键查询发布任务，记录不存在时统一转换为 ErrNotFound
func (r *Repo) GetDeployment(ctx context.Context, id int64) (*model.Deployment, error) {
	var dep model.Deployment
	err := r.db.WithContext(ctx).First(&dep, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &dep, err
}

// ListDeployments 按筛选条件返回一页发布任务及总数
func (r *Repo) ListDeployments(ctx context.Context, filter DeploymentFilter) ([]model.Deployment, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Deployment{})
	if filter.AppID > 0 {
		query = query.Where("app_id = ?", filter.AppID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var deployments []model.Deployment
	err := query.Order("id DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&deployments).Error
	return deployments, total, err
}

// TransitionStatus 校验状态机后执行 from 到 to 的条件更新，extra 是随状态一起写入的附加字段
// 返回值表示是否命中当前状态并完成更新，false 和 nil 代表 CAS 竞争失败而非数据库错误
func (r *Repo) TransitionStatus(ctx context.Context, id int64, from, to string, extra map[string]any) (bool, error) {
	if !status.CanTransition(status.Status(from), status.Status(to)) {
		return false, fmt.Errorf("非法状态转移: %s -> %s", from, to)
	}
	updates := map[string]any{
		"status":     to,
		"updated_at": time.Now(),
	}
	for key, value := range extra {
		updates[key] = value
	}
	result := r.db.WithContext(ctx).Model(&model.Deployment{}).
		Where("id = ? AND status = ?", id, from).
		Updates(updates)
	return result.RowsAffected > 0, result.Error
}

// TransitionExpiredLease 回收指定 attempt 的过期 running 任务并写入附加字段
// 返回值表示回收是否生效，更新条件会在写入瞬间再次校验租约避免回收刚续期的任务
func (r *Repo) TransitionExpiredLease(ctx context.Context, id int64, attempt int, to string, extra map[string]any) (bool, error) {
	if !status.CanTransition(status.Running, status.Status(to)) {
		return false, fmt.Errorf("非法状态转移: %s -> %s", status.Running, to)
	}
	updates := map[string]any{
		"status":     to,
		"updated_at": time.Now(),
	}
	for key, value := range extra {
		updates[key] = value
	}
	result := r.db.WithContext(ctx).Model(&model.Deployment{}).
		Where("id = ? AND status = ? AND attempt = ? AND lease_expires_at < ?", id, status.Running, attempt, time.Now()).
		Updates(updates)
	return result.RowsAffected > 0, result.Error
}

// RenewLease 仅为仍处于 running 且 owner 匹配的任务延长数据库租约
// 返回 false 和 nil 表示任务已换轮次、换持有者或离开 running
func (r *Repo) RenewLease(ctx context.Context, id int64, owner string, expiresAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.Deployment{}).
		Where("id = ? AND status = ? AND lease_owner = ?", id, status.Running, owner).
		Updates(map[string]any{
			"lease_expires_at": expiresAt,
			"updated_at":       time.Now(),
		})
	return result.RowsAffected > 0, result.Error
}

// FinalizeDeployment 将 running 任务写入终态，并在成功发布时同步应用当前版本和审计事件
// owner 与 attempt 标识执行轮次，返回值表示当前 Worker 是否仍有资格完成事务
func (r *Repo) FinalizeDeployment(ctx context.Context, deploymentID, appID, versionID int64, owner string, attempt int, result status.Status, reason, actor string) (bool, error) {
	if !status.CanTransition(status.Running, result) || result == status.Queued {
		return false, fmt.Errorf("非法任务终态: %s", result)
	}
	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// lease 未过期、owner 和 attempt 都匹配时才允许旧 Worker 写入终态
		statusUpdate := tx.Model(&model.Deployment{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND attempt = ? AND lease_expires_at >= ?", deploymentID, status.Running, owner, attempt, time.Now()).
			Updates(map[string]any{
				"status":           result,
				"fail_reason":      reason,
				"lease_owner":      nil,
				"lease_expires_at": nil,
				"updated_at":       time.Now(),
			})
		if statusUpdate.Error != nil || statusUpdate.RowsAffected == 0 {
			return statusUpdate.Error
		}
		updated = true

		// 只有成功发布才能推进应用当前版本，失败和回滚保持旧版本
		if result == status.Success {
			appUpdate := tx.Model(&model.App{}).
				Where("id = ?", appID).
				Update("current_version_id", versionID)
			if appUpdate.Error != nil {
				return appUpdate.Error
			}
			if appUpdate.RowsAffected == 0 {
				return ErrNotFound
			}
		}
		// 状态、当前版本和审计事件必须在同一事务中提交
		from, to := string(status.Running), string(result)
		return tx.Create(&model.Event{
			DeploymentID: deploymentID,
			Type:         "state_changed",
			FromStatus:   &from,
			ToStatus:     &to,
			Actor:        actor,
		}).Error
	})
	return updated && err == nil, err
}

// FindExpiredLeases 返回仍在 running 但数据库租约已经过期的任务
func (r *Repo) FindExpiredLeases(ctx context.Context) ([]model.Deployment, error) {
	var deployments []model.Deployment
	err := r.db.WithContext(ctx).
		Where("status = ? AND lease_expires_at < ?", status.Running, time.Now()).
		Find(&deployments).Error
	return deployments, err
}

// FindStalePending 返回创建时间早于 before 且仍未完成 MQ 确认的 pending 任务
func (r *Repo) FindStalePending(ctx context.Context, before time.Time) ([]model.Deployment, error) {
	var deployments []model.Deployment
	err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", status.Pending, before).
		Find(&deployments).Error
	return deployments, err
}

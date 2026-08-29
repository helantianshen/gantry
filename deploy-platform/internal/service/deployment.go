package service

import (
	"context"
	"errors"
	"fmt"
	"gantry/deploy-platform/internal/model"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/status"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAppNotFound        = errors.New("应用不存在")
	ErrVersionNotFound    = errors.New("版本不存在")
	ErrVersionAppMismatch = errors.New("版本不属于该应用")
	ErrPublish            = errors.New("消息发布失败")
	ErrQueueState         = errors.New("消息已发布，但任务状态更新失败")
)

// DeploymentService 编排 API 发起发布的业务流程
type DeploymentService struct {
	repo      *repo.Repo
	publisher *mq.Publisher
}

// NewDeploymentService 组装发布创建所需的持久化和消息发布依赖
func NewDeploymentService(r *repo.Repo, publisher *mq.Publisher) *DeploymentService {
	return &DeploymentService{
		repo:      r,
		publisher: publisher,
	}
}

// CreateDeployment 校验应用和版本后创建任务、发布消息并推进到 queued
// 成功时返回 queued 任务，失败时返回可供 API 映射的领域错误
func (s *DeploymentService) CreateDeployment(ctx context.Context, appID, versionID int64) (*model.Deployment, error) {
	app, err := s.repo.GetApp(ctx, appID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrAppNotFound
	}
	if err != nil {
		return nil, err
	}

	version, err := s.repo.GetVersion(ctx, versionID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, ErrVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	if version.AppID != appID {
		return nil, ErrVersionAppMismatch
	}

	// 当前版本在任务创建时固化，后续发布成功不会改变本任务的回滚依据
	var fromVersionID int64
	if app.CurrentVersionID != nil {
		fromVersionID = *app.CurrentVersionID
	}

	messageID := uuid.NewString()
	dep, err := s.repo.CreateDeployment(ctx, appID, versionID, fromVersionID, messageID)
	if err != nil {
		return nil, err
	}

	msg := mq.DeployMessage{
		MessageID:    messageID,
		DeploymentID: dep.ID,
		AppID:        appID,
		VersionID:    versionID,
	}
	publishCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// ponytail: 数据库提交后再发布，MVP 用失败终态收口；需要最终一致投递时改用 outbox
	if err := s.publisher.Publish(publishCtx, msg); err != nil {
		// 发布失败时尽力写入失败终态和事件，补偿再次失败由 stale pending 恢复流程兜底
		_, _ = s.repo.TransitionStatus(ctx, dep.ID, string(status.Pending), string(status.Failed), map[string]any{
			"fail_reason": "publish_failed: " + err.Error(),
		})
		_ = s.repo.InsertEvent(ctx, dep.ID, "state_changed", string(status.Pending), string(status.Failed), "api", nil)
		return nil, fmt.Errorf("%w: %v", ErrPublish, err)
	}

	ok, err := s.repo.TransitionStatus(ctx, dep.ID, string(status.Pending), string(status.Queued), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueueState, err)
	}
	if !ok {
		// 消息已发布但任务已不在 pending，不能把未知并发结果伪装成成功
		return nil, ErrQueueState
	}
	_ = s.repo.InsertEvent(ctx, dep.ID, "state_changed", string(status.Pending), string(status.Queued), "api", nil)
	dep.Status = string(status.Queued)
	return dep, nil
}

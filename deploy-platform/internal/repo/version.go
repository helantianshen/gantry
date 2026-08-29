package repo

import (
	"context"
	"errors"

	"gantry/deploy-platform/internal/model"

	"gorm.io/gorm"
)

// CreateVersion 为 appID 保存镜像版本，返回包含数据库回填字段的实体
func (r *Repo) CreateVersion(ctx context.Context, appID int64, tag, description string) (*model.Version, error) {
	version := &model.Version{
		AppID:       appID,
		Tag:         tag,
		Description: description,
	}
	return version, r.db.WithContext(ctx).Create(version).Error
}

// GetVersion 按主键查询版本，记录不存在时统一转换为 ErrNotFound
func (r *Repo) GetVersion(ctx context.Context, id int64) (*model.Version, error) {
	var version model.Version
	err := r.db.WithContext(ctx).First(&version, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &version, err
}

// ListVersions 按创建时间倒序返回指定应用的全部版本
func (r *Repo) ListVersions(ctx context.Context, appID int64) ([]model.Version, error) {
	// ponytail: 单应用版本记录明显增长后再增加分页
	var versions []model.Version
	err := r.db.WithContext(ctx).
		Where("app_id = ?", appID).
		Order("id DESC").
		Find(&versions).Error
	return versions, err
}

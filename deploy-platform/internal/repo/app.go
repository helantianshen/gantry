package repo

import (
	"context"
	"errors"

	"gantry/deploy-platform/internal/model"

	"gorm.io/gorm"
)

// CreateApp 持久化应用及其镜像和健康检查配置，返回包含数据库回填字段的实体
func (r *Repo) CreateApp(ctx context.Context, name, imageName, healthPath string, healthTimeout int) (*model.App, error) {
	app := &model.App{
		Name:                  name,
		ImageName:             imageName,
		HealthcheckPath:       healthPath,
		HealthcheckTimeoutSec: healthTimeout,
	}
	return app, r.db.WithContext(ctx).Create(app).Error
}

// GetApp 按主键查询应用，记录不存在时统一转换为 ErrNotFound
func (r *Repo) GetApp(ctx context.Context, id int64) (*model.App, error) {
	var app model.App
	err := r.db.WithContext(ctx).First(&app, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &app, err
}

// ListApps 按创建时间倒序返回一页应用及总数
func (r *Repo) ListApps(ctx context.Context, page, pageSize int) ([]model.App, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.App{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var apps []model.App
	err := query.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&apps).Error
	return apps, total, err
}

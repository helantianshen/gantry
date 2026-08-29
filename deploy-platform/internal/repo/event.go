package repo

import (
	"context"
	"gantry/deploy-platform/internal/model"
)

// ListEvents 按产生顺序返回 deploymentID 的全部审计事件
func (r *Repo) ListEvents(ctx context.Context, deploymentID int64) ([]model.Event, error) {
	var events []model.Event
	err := r.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("id ASC").
		Find(&events).Error
	return events, err
}

// InsertEvent 写入一条部署审计事件
// from 或 to 为空时保存为 NULL，detail 是可选的 JSON 原始数据
func (r *Repo) InsertEvent(ctx context.Context, deploymentID int64, eventType, from, to, actor string, detail []byte) error {
	event := &model.Event{
		DeploymentID: deploymentID,
		Type:         eventType,
		Actor:        actor,
		Detail:       detail,
	}
	if from != "" {
		event.FromStatus = &from
	}
	if to != "" {
		event.ToStatus = &to
	}
	return r.db.WithContext(ctx).Create(event).Error
}

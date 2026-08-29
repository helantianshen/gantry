package repo

import (
	"context"
	"gantry/deploy-platform/internal/model"
	"time"

	"gorm.io/gorm/clause"
)

// InsertIdempotencyKey 只记录首次消费，返回值表示本次是否成功插入新键
// 重复投递返回 false 和 nil，真正的执行权仍由任务状态 CAS 裁决
func (r *Repo) InsertIdempotencyKey(ctx context.Context, messageID string, deploymentID int64, consumer string) (bool, error) {
	now := time.Now()
	key := &model.IdempotencyKey{
		MessageID:    messageID,
		DeploymentID: deploymentID,
		Consumer:     consumer,
		ConsumedAt:   &now,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(key)
	return result.RowsAffected > 0, result.Error
}

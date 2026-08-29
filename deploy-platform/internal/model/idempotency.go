package model

import "time"

// IdempotencyKey 以 message_id 为主键记录首次消费
type IdempotencyKey struct {
	MessageID    string     `json:"message_id"`
	DeploymentID int64      `json:"deployment_id"`
	Consumer     string     `json:"consumer"`
	ConsumedAt   *time.Time `json:"consumed_at,omitempty"`
}

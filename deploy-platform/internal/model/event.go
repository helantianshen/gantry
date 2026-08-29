package model

import (
	"encoding/json"
	"time"
)

// Event 是只追加的发布审计记录
type Event struct {
	ID           int64           `json:"id"`
	DeploymentID int64           `json:"deployment_id"`
	Type         string          `json:"type"`
	FromStatus   *string         `json:"from_status,omitempty"`
	ToStatus     *string         `json:"to_status,omitempty"`
	Actor        string          `json:"actor"`
	Detail       json.RawMessage `json:"detail,omitempty"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
}

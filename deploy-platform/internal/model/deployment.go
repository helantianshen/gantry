package model

import "time"

// Deployment 是发布状态机的持久化实体
type Deployment struct {
	ID             int64      `json:"id"`
	AppID          int64      `json:"app_id"`
	VersionID      int64      `json:"version_id"`
	FromVersionID  *int64     `json:"from_version_id,omitempty"`
	Status         string     `json:"status"`
	MessageID      string     `json:"message_id"`
	LeaseOwner     *string    `json:"lease_owner,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	Attempt        int        `json:"attempt"`
	FailReason     string     `json:"fail_reason"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}

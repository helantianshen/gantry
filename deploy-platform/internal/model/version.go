package model

import "time"

// Version 对应 versions 表
type Version struct {
	ID          int64      `json:"id"`
	AppID       int64      `json:"app_id"`
	Tag         string     `json:"tag"`
	Description string     `json:"description"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

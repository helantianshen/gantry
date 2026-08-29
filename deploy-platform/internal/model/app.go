// Package model 定义与数据库表对应的领域数据
package model

import "time"

// App 对应 apps 表
type App struct {
	ID                    int64      `json:"id"`
	Name                  string     `json:"name"`
	ImageName             string     `json:"image_name"`
	HealthcheckPath       string     `json:"healthcheck_path"`
	HealthcheckTimeoutSec int        `json:"healthcheck_timeout_sec"`
	CurrentVersionID      *int64     `json:"current_version_id,omitempty"`
	CreatedAt             *time.Time `json:"created_at,omitempty"`
	UpdatedAt             *time.Time `json:"updated_at,omitempty"`
}

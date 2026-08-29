package repo

import (
	"errors"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("记录不存在")
var ErrActiveDeploy = errors.New("该应用存在活跃任务")

// Repo 是五个领域表共享的数据库入口
type Repo struct {
	db *gorm.DB
}

// New 使用 dsn 建立 MySQL 数据库入口，连接初始化失败时返回包装后的原因
func New(dsn string) (*Repo, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("MySQL 连接失败: %w", err)
	}
	return &Repo{
		db: db,
	}, nil
}

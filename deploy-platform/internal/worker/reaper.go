package worker

import (
	"context"
	"log"
	"time"

	"gantry/deploy-platform/internal/service"
)

// Reaper 只负责定时触发恢复服务
type Reaper struct {
	recovery *service.RecoveryService
	interval time.Duration
}

// NewReaper 创建每 30 秒触发一次恢复扫描的定时器
func NewReaper(recovery *service.RecoveryService) *Reaper {
	return &Reaper{
		recovery: recovery,
		interval: 30 * time.Second,
	}
}

// Run 周期调用恢复服务，ctx 取消时停止 ticker 并退出
func (r *Reaper) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	log.Printf("Reaper 启动, 扫描间隔 %s", r.interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.recovery.Recover(ctx)
		}
	}
}

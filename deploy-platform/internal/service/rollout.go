package service

import (
	"context"
	"fmt"
	"gantry/deploy-platform/internal/metrics"
	"gantry/deploy-platform/internal/model"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/status"
	"log"
	"time"
)

// rollout 拉取并启动目标版本，健康后才停止旧容器
// 返回值依次为发布终态和可持久化的失败原因，成功原因是空字符串
func (r *DeploymentRunner) rollout(ctx context.Context, msg mq.DeployMessage, app *model.App, version *model.Version) (status.Status, string) {
	oldID, err := r.exec.Inspect(ctx, msg.AppID, msg.DeploymentID)
	if err != nil {
		return status.Failed, "inspect old: " + err.Error()
	}
	if err := r.exec.Pull(ctx, app.ImageName, version.Tag); err != nil {
		return status.Failed, "pull: " + err.Error()
	}

	// 新旧容器使用不同的随机宿主端口，因此可以并行完成健康检查
	newID, addr, err := r.exec.Run(ctx, msg.AppID, msg.DeploymentID, app.ImageName, version.Tag)
	if err != nil {
		return r.keepOld(ctx, oldID, newID, "run: "+err.Error())
	}
	if addr == "" {
		return r.keepOld(ctx, oldID, newID, "run: 未返回健康地址")
	}
	target := fmt.Sprintf("http://%s%s", addr, app.HealthcheckPath)
	if err := r.checkHealth(ctx, target, time.Duration(app.HealthcheckTimeoutSec)*time.Second); err != nil {
		return r.keepOld(ctx, oldID, newID, "healthcheck: "+err.Error())
	}

	// 新容器健康后才切走旧容器
	if oldID != "" {
		if err := r.exec.Stop(ctx, oldID); err != nil {
			return r.keepOld(ctx, oldID, newID, "stop old: "+err.Error())
		}
		if err := r.exec.Remove(ctx, oldID); err != nil {
			log.Printf("清理已停止的旧容器失败: %v", err)
		}
	}
	return status.Success, ""
}

// keepOld 清理失败的新容器并保留旧容器
// 返回 RolledBack 表示旧容器仍可服务，FailedRollback 表示新容器也未能清理
func (r *DeploymentRunner) keepOld(ctx context.Context, oldID, newID, reason string) (status.Status, string) {
	if newID != "" {
		if err := r.exec.Remove(ctx, newID); err != nil {
			metrics.RollbackTotal.WithLabelValues("failed").Inc()
			return status.FailedRollback, reason + "; cleanup new: " + err.Error()
		}
	}
	if oldID != "" {
		metrics.RollbackTotal.WithLabelValues("success").Inc()
		return status.RolledBack, reason
	}
	return status.Failed, reason
}

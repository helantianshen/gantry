package service

import (
	"context"
	"errors"
	"gantry/deploy-platform/internal/executor"
	"gantry/deploy-platform/internal/lock"
	"gantry/deploy-platform/internal/metrics"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/status"
	"log"
	"time"
)

// ProcessingResult 告诉 Worker 应该确认、重投还是丢弃当前 RabbitMQ 消息
type ProcessingResult uint8

const (
	ProcessingComplete ProcessingResult = iota
	ProcessingRetry
	ProcessingDiscard
)

// deploymentAction 根据数据库状态决定本次消息应提前确认、重投或继续执行
type deploymentAction uint8

const (
	actionAcknowledge deploymentAction = iota
	actionRequeue
	actionExecute
)

// DeploymentRunner 执行单个发布任务
type DeploymentRunner struct {
	id          string
	repo        *repo.Repo
	exec        executor.Executor
	appLock     *lock.AppLock
	checkHealth func(context.Context, string, time.Duration) error
}

// NewDeploymentRunner 组装单任务执行器，id 用于审计 actor 和幂等消费记录
func NewDeploymentRunner(id string, r *repo.Repo, exec executor.Executor, appLock *lock.AppLock) *DeploymentRunner {
	healthChecker := executor.NewHealthChecker(5 * time.Second)
	return &DeploymentRunner{
		id:          id,
		repo:        r,
		exec:        exec,
		appLock:     appLock,
		checkHealth: healthChecker.Check,
	}
}

// Handle 处理一条部署消息并返回 Worker 的 Ack、重投或丢弃决策
// 返回值不直接操作 RabbitMQ，使业务编排与 AMQP delivery 保持解耦
func (r *DeploymentRunner) Handle(ctx context.Context, msg mq.DeployMessage) ProcessingResult {
	// 幂等键记录首次到达，重复消息仍需读取任务状态让 CAS 裁决执行权
	inserted, err := r.repo.InsertIdempotencyKey(ctx, msg.MessageID, msg.DeploymentID, r.id)
	if err != nil {
		log.Printf("写入幂等键失败: %v", err)
		return ProcessingRetry
	}
	if !inserted {
		log.Printf("收到重复消息，由任务状态 CAS 裁决: message_id=%s", msg.MessageID)
	}

	dep, err := r.repo.GetDeployment(ctx, msg.DeploymentID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ProcessingDiscard
		}
		return ProcessingRetry
	}
	switch actionFor(status.Status(dep.Status)) {
	case actionRequeue:
		// Publisher confirm 可能早于 pending → queued，短退避后让 RabbitMQ 重投
		select {
		case <-ctx.Done():
		case <-time.After(250 * time.Millisecond):
		}
		return ProcessingRetry
	case actionAcknowledge:
		// running 或终态消息已无需再次执行，记录检查事件后直接确认
		_ = r.repo.InsertEvent(ctx, dep.ID, "state_checked", dep.Status, dep.Status, r.actor(), nil)
		return ProcessingComplete
	}

	token, err := r.acquireLock(ctx, msg.AppID)
	if err != nil {
		return ProcessingRetry
	}
	defer func() {
		// 请求取消不应阻止释放 Redis 锁，因此使用独立且有界的后台上下文
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = r.appLock.Release(releaseCtx, msg.AppID, token)
	}()

	leaseExpires := time.Now().Add(90 * time.Second)
	// queued → running CAS 是唯一执行权裁决，Redis 锁只负责减少同应用并发竞争
	ok, err := r.repo.TransitionStatus(ctx, dep.ID, string(status.Queued), string(status.Running), map[string]any{
		"lease_owner":      token,
		"lease_expires_at": leaseExpires,
		"attempt":          msg.Attempt,
	})
	if err != nil {
		return ProcessingRetry
	}
	if !ok {
		// 其他 Worker 已推进状态，本条重复消息可以安全确认
		return ProcessingComplete
	}
	_ = r.repo.InsertEvent(ctx, dep.ID, "state_changed", string(status.Queued), string(status.Running), r.actor(), nil)

	runCtx, cancelRun := context.WithCancel(ctx)
	// 任一租约续期失败都会取消容器执行上下文，阻止失去所有权的 Worker 继续工作
	go r.leaseHeartbeat(runCtx, cancelRun, dep.ID, msg.AppID, token)
	defer cancelRun()

	app, appErr := r.repo.GetApp(runCtx, msg.AppID)
	version, versionErr := r.repo.GetVersion(runCtx, msg.VersionID)
	started := time.Now()
	result, reason := status.Failed, "读取应用或版本失败"
	if appErr == nil && versionErr == nil {
		result, reason = r.rollout(runCtx, msg, app, version)
	}

	// owner 和 attempt 是数据库终态栅栏，过期 Worker 恢复后不能覆盖新一轮结果
	ok, err = r.repo.FinalizeDeployment(
		ctx, dep.ID, msg.AppID, msg.VersionID,
		token, msg.Attempt, result, reason, r.actor())
	if err != nil || !ok {
		log.Printf("写入任务终态失败: deployment_id=%d ok=%v err=%v", dep.ID, ok, err)
	} else {
		metrics.DeploymentsTotal.WithLabelValues(string(result)).Inc()
		metrics.DeployDuration.WithLabelValues(string(result)).Observe(time.Since(started).Seconds())
	}
	return ProcessingComplete
}

// actionFor 将持久化状态映射为消息处理动作，只有 queued 可以进入真实执行流程
func actionFor(current status.Status) deploymentAction {
	switch current {
	case status.Pending:
		return actionRequeue
	case status.Queued:
		return actionExecute
	default:
		return actionAcknowledge
	}
}

// acquireLock 最多尝试五次获取应用锁，成功时返回同时用于数据库 lease 的唯一 token
func (r *DeploymentRunner) acquireLock(ctx context.Context, appID int64) (string, error) {
	for i := 0; i < 5; i++ {
		token, err := r.appLock.Acquire(ctx, appID, r.id)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, lock.ErrLockNotAcquired) {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return "", lock.ErrLockNotAcquired
}

// leaseHeartbeat 每 30 秒同时续期 Redis 锁和 MySQL lease
// cancelRun 在任一续期失败时停止当前执行，token 标识本轮唯一持有者
func (r *DeploymentRunner) leaseHeartbeat(ctx context.Context, cancelRun context.CancelFunc, deploymentID, appID int64, token string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.appLock.Renew(ctx, appID, token); err != nil {
				log.Printf("Redis 锁续期失败: %v", err)
				cancelRun()
				return
			}
			ok, err := r.repo.RenewLease(ctx, deploymentID, token, time.Now().Add(90*time.Second))
			if err != nil || !ok {
				log.Printf("数据库租约续期失败: ok=%v, err=%v", ok, err)
				cancelRun()
				return
			}
		}
	}
}

// actor 返回审计事件中可识别当前 Worker 的操作者名称
func (r *DeploymentRunner) actor() string {
	return "worker-" + r.id
}

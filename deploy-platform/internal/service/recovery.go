package service

import (
	"context"
	"gantry/deploy-platform/internal/metrics"
	"gantry/deploy-platform/internal/model"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/status"
	"log"
	"time"
)

// RecoveryService 回收发布和消费过程中的超时任务
type RecoveryService struct {
	repo       *repo.Repo
	publisher  *mq.Publisher
	maxAttempt int
}

// NewRecoveryService 创建故障恢复服务，默认允许初次执行后最多重投三次
func NewRecoveryService(r *repo.Repo, publisher *mq.Publisher) *RecoveryService {
	return &RecoveryService{
		repo:       r,
		publisher:  publisher,
		maxAttempt: 3,
	}
}

// Recover 执行一轮恢复，先收口未完成发布的 pending，再回收 running 过期租约
func (s *RecoveryService) Recover(ctx context.Context) {
	s.failStalePending(ctx)

	expired, err := s.repo.FindExpiredLeases(ctx)
	if err != nil {
		log.Printf("Reaper 扫描失败: %v", err)
		return
	}
	for i := range expired {
		s.reclaim(ctx, &expired[i])
	}
}

// failStalePending 将超过发布确认窗口的 pending 任务标记为 failed
func (s *RecoveryService) failStalePending(ctx context.Context) {
	stale, err := s.repo.FindStalePending(ctx, time.Now().Add(-30*time.Second))
	if err != nil {
		log.Printf("Reaper 扫描 pending 失败: %v", err)
		return
	}
	for i := range stale {
		ok, transitionErr := s.repo.TransitionStatus(ctx, stale[i].ID, string(status.Pending), string(status.Failed), map[string]any{
			"fail_reason": "publish_timeout",
		})
		if transitionErr == nil && ok {
			_ = s.repo.InsertEvent(ctx, stale[i].ID, "publish_timeout", string(status.Pending), string(status.Failed), "reaper", nil)
		}
	}
}

// reclaim 回收一条过期 running 任务，重试额度耗尽则失败，否则递增 attempt 并重新发布
func (s *RecoveryService) reclaim(ctx context.Context, dep *model.Deployment) {
	log.Printf("Reaper 回收过期任务: deployment_id=%d, attempt=%d", dep.ID, dep.Attempt)

	if dep.Attempt >= s.maxAttempt {
		// 当前 attempt 已耗尽重投额度，终止任务并清空旧 Worker 的租约信息
		ok, err := s.repo.TransitionExpiredLease(ctx, dep.ID, dep.Attempt, string(status.Failed), map[string]any{
			"fail_reason":      "lease_exhausted",
			"lease_owner":      nil,
			"lease_expires_at": nil,
		})
		if err != nil || !ok {
			log.Printf("Reaper 标记失败未生效: deployment_id=%d ok=%v err=%v", dep.ID, ok, err)
			return
		}
		metrics.LeaseReclaimTotal.Inc()
		_ = s.repo.InsertEvent(ctx, dep.ID, "lease_exhausted", string(status.Running), string(status.Failed), "reaper", nil)
		return
	}

	nextAttempt := dep.Attempt + 1
	// attempt 和租约过期条件共同防止多个 Reaper 重复回收同一轮任务
	ok, err := s.repo.TransitionExpiredLease(ctx, dep.ID, dep.Attempt, string(status.Queued), map[string]any{
		"lease_owner":      nil,
		"lease_expires_at": nil,
		"attempt":          nextAttempt,
	})
	if err != nil || !ok {
		log.Printf("Reaper 抢占过期任务失败: deployment_id=%d ok=%v err=%v", dep.ID, ok, err)
		return
	}
	metrics.LeaseReclaimTotal.Inc()
	_ = s.repo.InsertEvent(ctx, dep.ID, "lease_reclaimed", string(status.Running), string(status.Queued), "reaper", nil)
	if err := s.publisher.Publish(ctx, mq.DeployMessage{
		DeploymentID: dep.ID,
		AppID:        dep.AppID,
		VersionID:    dep.VersionID,
		MessageID:    dep.MessageID,
		Attempt:      nextAttempt,
	}); err != nil {
		s.retryLater(ctx, dep.ID, err)
		return
	}
	metrics.MQRetryTotal.WithLabelValues("lease_expired").Inc()
	log.Printf("任务重新排队: deployment_id=%d, attempt=%d→%d", dep.ID, dep.Attempt, nextAttempt)
}

// retryLater 在重新发布失败时制造一个短租约，让下一轮 Reaper 再次接管
// cause 仅用于日志保留本次发布失败原因
func (s *RecoveryService) retryLater(ctx context.Context, deploymentID int64, cause error) {
	_, err := s.repo.TransitionStatus(ctx, deploymentID, string(status.Queued), string(status.Running), map[string]any{
		"lease_owner":      "reaper-retry",
		"lease_expires_at": time.Now().Add(5 * time.Second),
	})
	log.Printf("Reaper 重投失败，稍后重试: deployment_id=%d cause=%v reset_err=%v", deploymentID, cause, err)
}

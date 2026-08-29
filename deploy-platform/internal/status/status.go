// Package status 定义部署状态及合法转移
package status

import "slices"

type Status string

const (
	Pending        Status = "pending"         // 发布任务已写入数据库，但 MQ 发布结果尚未确认
	Queued         Status = "queued"          // RabbitMQ 已通过 publisher confirm 接管消息，数据库已转为 queued
	Running        Status = "running"         //  Worker 已通过 queued → running CAS 获得执行权，并持有 lease
	Success        Status = "success"         // 新容器健康、旧容器已停止，应用当前版本已更新
	Failed         Status = "failed"          // 发布最终失败，并且没有完成“保留旧容器”的回滚结果
	RolledBack     Status = "rolled_back"     // 新容器发布失败但已被删除，旧容器始终保留运行
	FailedRollback Status = "failed_rollback" // 新容器发布失败，并且清理新容器也失败
)

// running → queued 只供 Reaper 回收过期租约后重入
var transitions = map[Status][]Status{
	// Pending -> Queued 消息发布成功
	// Pending -> Failed 消息发布失败
	Pending: {Queued, Failed},
	// Queued -> Running 任务开始执行
	Queued: {Running},
	// Running -> Success 任务执行成功
	// Running -> Failed 任务执行失败
	// Running -> RolledBack 容器发布失败，已回滚版本
	// Running -> FailedRollback 容器发布失败，版本回滚也失败
	// Running -> Queued 容器发布结果未知，被 Reaper 回收重新等待执行
	Running: {Success, Failed, RolledBack, FailedRollback, Queued},
}

// CanTransition 判断状态机是否允许从 from 转移到 to
func CanTransition(from, to Status) bool {
	return slices.Contains(transitions[from], to)
}

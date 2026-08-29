package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DeploymentsTotal 按状态统计发布结果
	DeploymentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "deploy_platform_deployments_total",
		Help: "发布任务总数（按状态分类）",
	}, []string{"status"})

	// DeployDuration 记录发布耗时
	DeployDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "deploy_platform_deploy_duration_seconds",
		Help:    "单次发布耗时（秒）",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
	}, []string{"result"})

	// WorkerActive 记录当前活跃 Worker 数
	WorkerActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "deploy_platform_worker_active",
		Help: "当前活跃 Worker 数",
	})

	// QueueDepth 记录 RabbitMQ 待消费消息数
	QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "deploy_platform_queue_depth",
		Help: "RabbitMQ 待消费消息数",
	})

	// QueueConsumers 记录 RabbitMQ 当前消费者数
	QueueConsumers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "deploy_platform_queue_consumers",
		Help: "RabbitMQ 当前消费者数",
	})

	// MQRetryTotal 记录 MQ 重投次数
	MQRetryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "deploy_platform_mq_retry_total",
		Help: "MQ 消息重投次数",
	}, []string{"reason"})

	// LeaseReclaimTotal 记录 lease 回收次数
	LeaseReclaimTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "deploy_platform_lease_reclaim_total",
		Help: "lease 过期回收次数",
	})

	// RollbackTotal 记录回滚次数
	RollbackTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "deploy_platform_rollback_total",
		Help: "回滚次数（按结果分类）",
	}, []string{"result"})
)

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"gantry/deploy-platform/internal/executor"
	"gantry/deploy-platform/internal/lock"
	"gantry/deploy-platform/internal/metrics"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/service"
	"gantry/deploy-platform/internal/worker"
)

// main 组装 Worker、Reaper 和指标服务，并统一管理进程生命周期
func main() {
	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%d", os.Getpid())
	}

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("未设置 MYSQL_DSN，请从 .env.example 创建本地 .env")
	}
	mqURL := os.Getenv("RABBITMQ_URL")
	if mqURL == "" {
		log.Fatal("未设置 RABBITMQ_URL，请从 .env.example 创建本地 .env")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("未设置 REDIS_URL，请从 .env.example 创建本地 .env")
	}
	instance := os.Getenv("GANTRY_INSTANCE")
	if instance == "" {
		instance = "gantry"
	}

	r, err := repo.New(dsn)
	if err != nil {
		log.Fatalf("DB 初始化失败: %v", err)
	}

	conn, ch, err := mq.NewConn(mqURL)
	if err != nil {
		log.Fatalf("MQ 连接失败: %v", err)
	}
	defer conn.Close()
	defer ch.Close()

	cons, err := mq.NewConsumer(ch, workerID)
	if err != nil {
		log.Fatalf("消费者初始化失败: %v", err)
	}
	defer cons.Close()

	// 消费和恢复重投使用独立通道，避免 publisher confirm 干扰消费通道
	pubCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("发布通道初始化失败: %v", err)
	}
	defer pubCh.Close()
	pub, err := mq.NewPublisher(pubCh)
	if err != nil {
		log.Fatalf("发布者初始化失败: %v", err)
	}
	// 队列指标使用独立通道，避免 QueueInspect 与 Consume 的协议响应串线
	inspectCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("指标通道初始化失败: %v", err)
	}
	defer inspectCh.Close()

	rOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Redis URL 解析失败: %v", err)
	}
	rdb := redis.NewClient(rOpts)
	defer rdb.Close()
	appLock := lock.NewAppLock(rdb)

	exec, err := executor.NewDockerOps(instance)
	if err != nil {
		log.Fatalf("Docker 初始化失败: %v", err)
	}

	runner := service.NewDeploymentRunner(workerID, r, exec, appLock)
	w := worker.New(workerID, cons, runner)
	recovery := service.NewRecoveryService(r, pub)
	reaper := worker.NewReaper(recovery)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go reaper.Run(ctx)
	// RabbitMQ 队列深度和消费者数没有推送接口，按固定周期主动采集
	go func() {
		for {
			queue, err := inspectCh.QueueInspect(mq.QueueName)
			if err != nil {
				log.Printf("读取队列指标失败: %v", err)
				return
			}
			metrics.QueueDepth.Set(float64(queue.Messages))
			metrics.QueueConsumers.Set(float64(queue.Consumers))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()

	go func() {
		// Worker 消费循环异常时通知主协程结束整个进程
		if err := w.Run(ctx); err != nil {
			errCh <- fmt.Errorf("Worker: %w", err)
		}
	}()

	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":9090"
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{Addr: metricsAddr, Handler: metricsMux}
	// 指标服务独立于消费循环运行，异常同样触发进程关闭
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics: %w", err)
		}
	}()

	log.Printf("Worker %s 启动完成，消费队列: %s", workerID, mq.QueueName)

	select {
	case <-ctx.Done():
	case err := <-errCh:
		log.Printf("Worker 异常退出: %v", err)
	}
	log.Printf("Worker %s 正在关闭...", workerID)
	stop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = metricsServer.Shutdown(shutdownCtx)
	log.Printf("Worker %s 已关闭", workerID)
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gantry/deploy-platform/internal/api"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/repo"
	"gantry/deploy-platform/internal/service"
)

// main 组装 API 进程依赖并在系统信号或服务异常时优雅关闭 HTTP 服务
func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("未设置 MYSQL_DSN，请从 .env.example 创建本地 .env")
	}
	mqURL := os.Getenv("RABBITMQ_URL")
	if mqURL == "" {
		log.Fatal("未设置 RABBITMQ_URL，请从 .env.example 创建本地 .env")
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

	pub, err := mq.NewPublisher(ch)
	if err != nil {
		log.Fatalf("发布者初始化失败: %v", err)
	}

	rGin := gin.Default()
	deployments := service.NewDeploymentService(r, pub)
	h := api.New(r, deployments)
	h.Register(rGin)
	rGin.GET("/metrics", gin.WrapH(promhttp.Handler()))
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: rGin,
	}

	serverErr := make(chan error, 1)
	// HTTP 服务异步运行，非正常退出通过缓冲通道通知主协程
	go func() {
		log.Printf("API 服务启动: %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
	case err := <-serverErr:
		log.Printf("API 服务异常: %v", err)
	}
	log.Println("API 正在关闭...")
	// 关闭预算避免连接长期阻塞进程退出
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("API 关闭警告: %v", err)
	}
	log.Println("API 已关闭")
}

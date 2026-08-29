package executor

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Executor 隔离 Docker SDK，测试使用最小 fake
type Executor interface {
	// Pull 确保指定镜像和标签在本机可用
	Pull(ctx context.Context, imageName, tag string) error
	// Stop 优雅停止指定容器，空 ID 视为无需操作
	Stop(ctx context.Context, containerID string) error
	// Remove 强制删除指定容器，空 ID 视为无需操作
	Remove(ctx context.Context, containerID string) error
	// Run 启动新版本容器，返回容器 ID、宿主机健康检查地址和错误
	Run(ctx context.Context, appID, deploymentID int64, imageName, tag string) (containerID, containerIP string, err error)
	// Inspect 查找当前应用唯一的旧稳定容器，没有旧容器时返回空 ID
	Inspect(ctx context.Context, appID, deploymentID int64) (string, error)
}

// HealthChecker 在发布预算内轮询新容器
type HealthChecker struct {
	client   *http.Client
	interval time.Duration
}

// NewHealthChecker 创建健康检查器，timeout 是单次 HTTP 请求超时而非整个发布预算
func NewHealthChecker(timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: nil,
			},
		},
		interval: 2 * time.Second,
	}
}

// Check 在 budget 内轮询 target，首次收到 2xx 或 3xx 响应即视为健康
func (hc *HealthChecker) Check(ctx context.Context, target string, budget time.Duration) error {
	checkCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	for {
		req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, target, nil)
		if err != nil {
			return fmt.Errorf("健康检查地址无效: %w", err)
		}
		resp, err := hc.client.Do(req)
		if err == nil {
			resp.Body.Close()
			// 重定向后的最终 3xx 仍表示服务已经能够响应请求
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
		select {
		case <-checkCtx.Done():
			return fmt.Errorf("健康检查失败 %s: %w", target, checkCtx.Err())
		case <-time.After(hc.interval):
			// 非健康响应在预算内继续轮询，不让单次失败立即终止发布
		}
	}
}

// 要求 *DockerOps 必须实现 Executor 接口，否则编译失败
var _ Executor = (*DockerOps)(nil)

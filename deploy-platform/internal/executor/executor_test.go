package executor

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestHealthCheckerRetriesUntilHealthy 验证首次失败后会在预算内继续轮询
func TestHealthCheckerRetriesUntilHealthy(t *testing.T) {
	// httptest 的请求处理器运行在独立 goroutine，使用原子计数避免并发读写竞争
	var calls atomic.Int32
	// NewServer 在本机随机端口启动临时 HTTP 服务，并通过 server.URL 暴露访问地址
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Add 返回自增后的值，第一次请求返回 503 模拟容器尚未就绪
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// 第二次请求返回 204，属于 2xx，HealthChecker 应将其判定为健康
		w.WriteHeader(http.StatusNoContent)
	}))
	// 测试结束时关闭监听端口，避免临时服务器泄漏
	defer server.Close()

	// 构造参数控制单次 HTTP 请求超时，不是整个健康检查的总时限
	checker := NewHealthChecker(time.Second)
	// 将默认两秒重试间隔缩短到一毫秒，让测试快速完成
	checker.interval = time.Millisecond
	// t.Context 在测试结束时自动取消，最后一个 time.Second 是本次轮询的总预算
	if err := checker.Check(t.Context(), server.URL, time.Second); err != nil {
		// 返回错误说明检查器没有在预算内从 503 重试到 204
		t.Fatal(err)
	}
	// 必须恰好请求两次，既证明发生了一次重试，也证明成功后立即停止轮询
	if calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", calls.Load())
	}
}

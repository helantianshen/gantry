package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gantry/deploy-platform/internal/model"
	"gantry/deploy-platform/internal/mq"
	"gantry/deploy-platform/internal/status"
)

type fakeExecutor struct {
	old     string
	stopped []string
	removed []string
}

// Pull 模拟镜像已就绪
func (f *fakeExecutor) Pull(context.Context, string, string) error { return nil }

// Run 返回固定的新容器和健康检查地址
func (f *fakeExecutor) Run(context.Context, int64, int64, string, string) (string, string, error) {
	return "new", "127.0.0.1:12345", nil
}

// Inspect 返回测试场景预设的旧容器
func (f *fakeExecutor) Inspect(context.Context, int64, int64) (string, error) {
	return f.old, nil
}

// Stop 记录被停止的容器 ID
func (f *fakeExecutor) Stop(_ context.Context, id string) error {
	f.stopped = append(f.stopped, id)
	return nil
}

// Remove 记录被删除的容器 ID
func (f *fakeExecutor) Remove(_ context.Context, id string) error {
	f.removed = append(f.removed, id)
	return nil
}

// TestRolloutKeepsOldUntilNewIsHealthy 验证健康失败保留旧容器且健康成功后才切换
func TestRolloutKeepsOldUntilNewIsHealthy(t *testing.T) {
	msg := mq.DeployMessage{AppID: 1, DeploymentID: 2}
	app := &model.App{ImageName: "demo", HealthcheckPath: "/", HealthcheckTimeoutSec: 1}
	version := &model.Version{Tag: "v2"}

	t.Run("health failure keeps old", func(t *testing.T) {
		exec := &fakeExecutor{old: "old"}
		runner := &DeploymentRunner{exec: exec, checkHealth: func(context.Context, string, time.Duration) error {
			return errors.New("unhealthy")
		}}
		got, _ := runner.rollout(context.Background(), msg, app, version)
		if got != status.RolledBack || len(exec.stopped) != 0 || len(exec.removed) != 1 || exec.removed[0] != "new" {
			t.Fatalf("result=%s stopped=%v removed=%v", got, exec.stopped, exec.removed)
		}
	})

	t.Run("health success switches containers", func(t *testing.T) {
		exec := &fakeExecutor{old: "old"}
		runner := &DeploymentRunner{exec: exec, checkHealth: func(context.Context, string, time.Duration) error { return nil }}
		got, _ := runner.rollout(context.Background(), msg, app, version)
		if got != status.Success || len(exec.stopped) != 1 || exec.stopped[0] != "old" || len(exec.removed) != 1 || exec.removed[0] != "old" {
			t.Fatalf("result=%s stopped=%v removed=%v", got, exec.stopped, exec.removed)
		}
	})
}

// TestActionFor 验证持久化状态到消息动作的映射
func TestActionFor(t *testing.T) {
	cases := map[status.Status]deploymentAction{
		status.Pending: actionRequeue,
		status.Queued:  actionExecute,
		status.Running: actionAcknowledge,
		status.Success: actionAcknowledge,
	}
	for input, want := range cases {
		if got := actionFor(input); got != want {
			t.Errorf("status %s: got action %d, want %d", input, got, want)
		}
	}
}

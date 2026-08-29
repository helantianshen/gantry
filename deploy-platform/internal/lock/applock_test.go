package lock

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestAppLockIntegration 验证锁竞争以及旧 token 不能删除新持有者的锁
func TestAppLockIntegration(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL 未设置")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	defer client.Close()

	ctx := context.Background()
	const appID int64 = 922337203685477580
	defer client.Del(ctx, lockKey(appID))
	appLock := NewAppLock(client)
	token, err := appLock.Acquire(ctx, appID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appLock.Acquire(ctx, appID, "worker-b"); !errors.Is(err, ErrLockNotAcquired) {
		t.Fatalf("竞争锁错误 = %v", err)
	}
	if err := client.Set(ctx, lockKey(appID), "worker-b:new", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err := appLock.Release(ctx, appID, token); err != nil {
		t.Fatal(err)
	}
	if got := client.Get(ctx, lockKey(appID)).Val(); got != "worker-b:new" {
		t.Fatalf("旧 token 误删新锁: %q", got)
	}
}

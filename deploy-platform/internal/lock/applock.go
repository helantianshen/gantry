package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrLockNotAcquired = errors.New("锁已被占用")

// token 校验防止过期 Worker 误删新锁
const releaseLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`

const renewLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("EXPIRE", KEYS[1], ARGV[2])
else
    return 0
end
`

// AppLock 用 Redis 串行化同一应用的发布
type AppLock struct {
	client *redis.Client
	ttl    time.Duration
}

// NewAppLock 创建应用级分布式锁，默认租期为 60 秒
func NewAppLock(client *redis.Client) *AppLock {
	return &AppLock{
		client: client,
		ttl:    60 * time.Second,
	}
}

// Acquire 尝试锁定 appID，成功时返回唯一 token，锁已占用时返回 ErrLockNotAcquired
// token 同时作为数据库 lease owner，不能只用 workerID 代替
func (al *AppLock) Acquire(ctx context.Context, appID int64, workerID string) (string, error) {
	token := fmt.Sprintf("%s:%d", workerID, time.Now().UnixNano())

	ok, err := al.client.SetNX(ctx, lockKey(appID), token, al.ttl).Result()
	if err != nil {
		return "", fmt.Errorf("Redis SetNX 失败: %w", err)
	}
	if !ok {
		return "", ErrLockNotAcquired
	}
	return token, nil
}

// Release 仅在 token 仍属于当前持有者时删除锁，避免旧 Worker 误删新锁
func (al *AppLock) Release(ctx context.Context, appID int64, token string) error {
	_, err := al.client.Eval(ctx, releaseLua, []string{lockKey(appID)}, token).Result()
	if err != nil {
		return fmt.Errorf("Lua 释放锁失败: %w", err)
	}
	return nil
}

// Renew 仅为 token 匹配的锁续期，锁已更换或过期时返回错误
func (al *AppLock) Renew(ctx context.Context, appID int64, token string) error {
	// 只有持有当前 token 的 Worker 才能续期
	result, err := al.client.Eval(ctx, renewLua, []string{lockKey(appID)}, token, int(al.ttl/time.Second)).Int()
	if err != nil {
		return fmt.Errorf("Lua 续期失败: %w", err)
	}
	if result == 0 {
		return errors.New("锁已释放或过期")
	}
	return nil
}

// lockKey 为每个应用生成独立的 Redis 锁键
func lockKey(appID int64) string {
	return fmt.Sprintf("app:lock:%d", appID)
}

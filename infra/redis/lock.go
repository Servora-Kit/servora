package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var (
	// ErrLockNotAcquired 表示锁已被其他持有者占用。
	ErrLockNotAcquired = errors.New("redis: lock not acquired")
	// ErrLockNotHeld 表示当前实例不持有该锁（已过期或 token 不匹配）。
	ErrLockNotHeld = errors.New("redis: lock not held")
)

// unlockScript 使用 Lua 脚本原子性地验证 token 并删除 key。
var unlockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)

// Lock 表示一个已获取的分布式锁。
type Lock struct {
	client *Client
	key    string
	token  string
}

// TryLock 尝试获取分布式锁。
// 基于 Redis SET NX 实现，使用随机 token 标识持有者。
// 如果锁已被占用，返回 ErrLockNotAcquired。
func (c *Client) TryLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	result, err := c.rdb.SetArgs(ctx, key, token, goredis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrLockNotAcquired
	}
	if err != nil {
		return nil, err
	}
	if result != "OK" {
		return nil, ErrLockNotAcquired
	}

	return &Lock{
		client: c,
		key:    key,
		token:  token,
	}, nil
}

// Unlock 释放分布式锁。
// 使用 Lua 脚本原子性地验证 token 并删除 key，防止误释放他人持有的锁。
// 如果锁已过期或 token 不匹配，返回 ErrLockNotHeld。
func (l *Lock) Unlock(ctx context.Context) error {
	result, err := unlockScript.Run(ctx, l.client.rdb, []string{l.key}, l.token).Int64()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

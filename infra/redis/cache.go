package redis

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// GetOrSet 实现 Cache-aside 模式。
// 查缓存 → 未命中调用 loader → 写回缓存。
// Redis 读取失败时降级为直接调用 loader（缓存故障不阻塞业务）。
func GetOrSet[T any](
	ctx context.Context,
	c *Client,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) (T, error),
	marshal func(T) (string, error),
	unmarshal func(string) (T, error),
) (T, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == nil {
		return unmarshal(val)
	}

	// Redis 故障（非 key 不存在）→ 降级直接调用 loader
	if err != goredis.Nil {
		return loader(ctx)
	}

	// 缓存未命中 → 调用 loader
	result, err := loader(ctx)
	if err != nil {
		return result, err
	}

	// 写回缓存（忽略写入错误，不影响业务返回）
	if serialized, marshalErr := marshal(result); marshalErr == nil {
		c.rdb.Set(ctx, key, serialized, ttl)
	}

	return result, nil
}

// GetOrSetJSON 是 GetOrSet 的 JSON 便捷版本。
// 内置 JSON 序列化/反序列化，适用于最常见的缓存场景。
func GetOrSetJSON[T any](
	ctx context.Context,
	c *Client,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) (T, error),
) (T, error) {
	return GetOrSet(ctx, c, key, ttl, loader,
		func(v T) (string, error) {
			b, err := json.Marshal(v)
			return string(b), err
		},
		func(s string) (T, error) {
			var v T
			err := json.Unmarshal([]byte(s), &v)
			return v, err
		},
	)
}

# AGENTS.md - pkg/redis/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 模块目的

封装 `github.com/redis/go-redis/v9`，统一配置转换、连通性探测、日志与基础操作，并提供分布式锁和 Cache-aside 高级模式。

## 当前实现事实

- 默认超时：`Dial=5s`、`Read=3s`、`Write=3s`
- `NewConfigFromProto` 从 `conf.Data_Redis` 构造本地配置
- `NewClient` 会先 `Ping` 校验连接，并返回 `cleanup func()`
- 初始化日志统一带 `module=redis/pkg`

## 暴露能力

- `Ping`
- `Set` / `Get` / `Del` / `Has` / `Keys`
- `SAdd` / `SMembers`
- `Expire`
- `TryLock` / `Lock.Unlock`：基于 SET NX + Lua 的分布式锁
- `GetOrSet[T]` / `GetOrSetJSON[T]`：Cache-aside 泛型 helper

## 使用示例

```go
cfg := &redis.Config{Addr: "localhost:6379", DB: 0}
client, cleanup, err := redis.NewClient(cfg, l)
defer cleanup()

_ = client.Set(context.Background(), "key", "value", time.Hour)
```

### 分布式锁

```go
lock, err := client.TryLock(ctx, "order:123:lock", 10*time.Second)
if err != nil { /* 锁已被占用或错误 */ }
defer lock.Unlock(ctx)
```

### Cache-aside

```go
user, err := redis.GetOrSetJSON[User](ctx, client, "user:1", time.Hour,
    func(ctx context.Context) (User, error) {
        return db.FindUser(ctx, 1)
    },
)
```

## 测试

```bash
go test ./pkg/redis/...
```

需要本地 Redis；不可用时应在测试里 `t.Skipf(...)`。

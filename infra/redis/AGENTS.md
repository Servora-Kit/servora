# AGENTS.md - infra/redis/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 模块目的

封装 `github.com/redis/go-redis/v9`，统一配置转换、连通性探测、日志与基础操作，并提供分布式锁和 Cache-aside 高级模式。

## 当前实现事实

- 默认超时：`Dial=5s`、`Read=3s`、`Write=3s`
- `NewConfigFromProto` 从 `conf.Data_Redis` 构造本地配置
- `NewClient` 会先 `Ping` 校验连接，并返回 `cleanup func()`
- 初始化日志统一带 `module=redis/infra`
- 当前一级目录没有单独测试文件，测试约定仍以包级 `go test` 为主

## 暴露能力

- `Ping`
- `Set` / `Get` / `Del` / `Has` / `Keys`
- `SAdd` / `SMembers`
- `Expire`
- `TryLock` / `Lock.Unlock`：基于 SET NX + Lua 的分布式锁
- `GetOrSet[T]` / `GetOrSetJSON[T]`：Cache-aside 泛型 helper

## 边界约束

- 本包负责 Redis 访问与常用模式封装，不负责业务缓存键设计或领域失效策略
- 不把具体业务对象序列化格式、事件语义或授权语义硬编码到共享 Redis 层
- 锁与 Cache-aside 是基础设施 helper，不是业务事务补偿框架

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

## 常见反模式

- 在 `infra/redis` 中硬编码业务 key 命名与对象 schema
- 误把 `GetOrSet` 当成可替代业务一致性策略的万能缓存方案
- 忽略 `cleanup` 或锁释放，造成连接/锁资源泄漏

## 测试

```bash
go test ./infra/redis/...
```

需要本地 Redis；不可用时应在测试里 `t.Skipf(...)`。

## 维护提示

- 若调整默认超时或连通性校验策略，需同步确认所有依赖方的启动容忍度
- 若扩展新的高级 helper，优先保持 API 通用，不为某个业务模型定制

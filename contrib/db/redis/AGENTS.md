# AGENTS.md - contrib/db/redis/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-08-21 -->

## 模块目的

封装 `github.com/redis/go-redis/v9` 的配置约定：将 `servora.contrib.db.redis.v1.Redis` 配置转换为官方客户端（含 TLS 构建与连通性探测），不自行封装 KV 操作，并提供通用分布式锁。

## 当前实现事实

- 默认超时由 Redis Proto 声明：`Dial=5s`、`Read=3s`、`Write=3s`；显式零值原样传给 go-redis，不在本包再次补默认
- `New` 直接接收 `servora.contrib.db.redis.v1.Redis`，先调用 `Apply()`，再将地址、网络类型及超时等设置传给官方客户端
- `New` 完成配置转换、TLS 构建和 `Ping` 连通性校验，并返回 `cleanup func()`
- 不记录构造期生命周期日志；业务日志由调用方自行拼装

## 暴露能力

- `New(cfg) (*redis.Client, func(), error)`：从 proto 配置构造官方客户端并校验连通性
- `TryLock(ctx, rdb, key, ttl)` / `Lock.Unlock`：基于 SET NX + Lua 的分布式锁
- Cache-aside helper 位于 `contrib/cache/redis`

KV 读写直接使用 go-redis 官方 API（`Get`/`Set`/`Del`/…），本包不复制一层包装。

## 边界约束

- 本包只负责配置转换与连接建立，不封装 KV API、不持有业务日志
- 不把具体业务对象序列化格式、事件语义或授权语义硬编码到共享 Redis 层
- 锁是基础设施 helper，不是业务事务补偿框架

## 使用示例

```go
cfg := &redispb.Redis{Addr: "localhost:6379", Db: 0}
client, cleanup, err := redis.New(cfg)
if err != nil { return err }
defer cleanup()

_ = client.Set(context.Background(), "key", "value", time.Hour).Err()
```

### 分布式锁

```go
lock, err := redis.TryLock(ctx, client, "order:123:lock", 10*time.Second)
if err != nil { /* 锁已被占用或错误 */ }
defer lock.Unlock(ctx)
```

## 常见反模式

- 在 `contrib/db/redis` 中重新封装 go-redis 的 KV 方法或硬编码业务 key 命名
- 忽略 `cleanup` 或锁释放，造成连接/锁资源泄漏

## 测试

```bash
go test ./contrib/db/redis/...
```

需要本地 Redis；不可用时应在测试里 `t.Skipf(...)`。

## 维护提示

- 若调整默认超时或连通性校验策略，需同步确认所有依赖方的启动容忍度
- 若扩展新的高级 helper，优先保持 API 通用，不为某个业务模型定制
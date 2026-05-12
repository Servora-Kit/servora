# AGENTS.md - transport/server/internal/

<!-- Parent: ../../AGENTS.md -->

## 目录定位

`transport/server/internal/` 收纳 **仅 server 侧使用** 的辅助代码。借助 Go `internal/` 机制，**只允许 `transport/server/` 子树内的包 import**。

## 当前成员

- `registry/` 组装服务注册端点 URL（`ResolveRegistryEndpoint`）
- `accept/` TCP `Accept` 循环辅助（`AcceptLoop`，处理 `net.ErrClosed` 与 retry）

## 准入标准

- **server-only**：仅被 `transport/server/{grpc,http,...}` 引用，不被 client 引用
- 主题清晰（包名一句话讲清职责）
- 不混入 RPC 业务逻辑

## 反模式

- 双向共享：应该归到 `transport/internal/`
- 单文件包凑成「util 桶」：拆主题或归并

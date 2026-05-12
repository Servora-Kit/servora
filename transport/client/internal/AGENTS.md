# AGENTS.md - transport/client/internal/

<!-- Parent: ../../AGENTS.md -->

## 目录定位

`transport/client/internal/` 收纳 **仅 client 侧使用** 的辅助代码。借助 Go `internal/` 机制，**只允许 `transport/client/` 子树内的包 import**。

## 当前成员

- `endpointindex/` 把 `conf.Data.Client.Services[]` 按 protocol 索引成 service → endpoint map（`BuildClientEndpointIndex`）

## 准入标准

- **client-only**：仅被 `transport/client/{grpc,http,...}` 引用，不被 server 引用
- 主题清晰（包名一句话讲清职责）
- 不混入 RPC 业务逻辑

## 反模式

- 双向共享：应该归到 `transport/internal/`
- 单文件包凑成「util 桶」：拆主题或归并

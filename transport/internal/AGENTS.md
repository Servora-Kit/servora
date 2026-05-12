# AGENTS.md - transport/internal/

<!-- Parent: ../AGENTS.md -->

## 目录定位

`transport/internal/` 收纳 transport 内部「**server 与 client 双向共享**」的辅助代码。借助 Go `internal/` 机制，**只允许 `transport/` 子树内的包 import**，外部 import 会被编译器拒绝。

## 当前成员

- `normalize/` 通用正规化工具（`NormalizeDuration` / `NormalizeEndpoint`）
- `tls/` 把 `conf.TLSConfig` 转 `*tls.Config` 的 builder（内部调用 `security/tlsutil` 的密码学原语）

## 准入标准

- 该辅助代码 **同时被 transport server 和 transport client 引用**
- 主题清晰（包名一句话讲清职责）
- 不混入业务语义、不混入 capability 特定逻辑

## 反模式

- 单侧使用（server 或 client 一方独用）：应该归到 `transport/server/internal/` 或 `transport/client/internal/`
- 无主题集合（`util/` / `helpers/`）：拆主题或归并

## 与 `security/tlsutil` 的关系

`transport/internal/tls` 是「conf-to-tls builder」（消费 `conf.TLSConfig`），内部用 `security/tlsutil` 的密码学原语（证书加载与校验）。两者职责不同，不合并。

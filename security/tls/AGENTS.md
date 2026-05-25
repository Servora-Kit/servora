# AGENTS.md - security/tls/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-05-21 -->

## 模块定位

`security/tls` 集中构造 server/client TLS 配置。transport、tracing 等调用方只消费这里产出的 `*crypto/tls.Config`，不要重复读取 PEM 或拼装证书池。

公共配置 proto 位于 `servora/security/tls/v1/config.proto`，message 为 `TLS`。server/client endpoint YAML 字段仍叫 `tls`。

## 公开 API

```go
type ServerConfigOptions struct {
    CertPath string
    KeyPath string
    MinVersion uint16
}

type ClientConfigOptions struct {
    CAPath string
    CertPath string
    KeyPath string
    ServerName string
    InsecureSkipVerify bool
    MinVersion uint16
}

func NewServerConfig(opts ServerConfigOptions) (*tls.Config, error)
func MustServerConfig(opts ServerConfigOptions) *tls.Config
func NewClientConfig(opts ClientConfigOptions) (*tls.Config, error)
func LoadCertPoolFromPEMFile(path string) (*x509.CertPool, error)
func BuildServerTLS(c *tlspb.TLS) (*tls.Config, error)
func BuildClientTLS(c *tlspb.TLS) (*tls.Config, error)
func MustBuildServerTLS(c *tlspb.TLS) *tls.Config
```

默认 `MinVersion` 是 TLS 1.2。调用方需要更高版本时显式设置；不要降低默认值。

## 执行语义

- server 配置要求 cert/key 同时存在，否则返回 `tls cert_path and key_path are required`。
- client mTLS 要求 cert/key 成对出现；只传一个返回错误。
- `CAPath` 非空时加载 PEM 到 `RootCAs`；PEM 无效返回错误。
- `InsecureSkipVerify` 只按配置透传；调用方负责限制使用场景。
- `MustServerConfig` 只用于启动期装配，错误直接 panic。

## 边界约束

- 不在 transport/server/client、obs/tracing、infra 目录复制证书读取逻辑。
- 本包只接受 `servora.security.tls.v1.TLS` 或 options，不读取业务配置文件。
- 不在本包做动态证书轮转、secret manager、ACME 或证书发现。
- 保持错误信息带操作上下文，便于启动失败定位。

## 测试

```bash
go test ./security/tls
```

重点覆盖：server 缺文件、client 默认值、CA + client cert、cert/key 配对校验、无效 PEM。

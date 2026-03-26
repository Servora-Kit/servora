# AGENTS.md - security/jwt/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供 JWT 核心基础库，负责 RS256 签发、验签、RSA PEM 解析、KID 生成与通用 claims context 辅助。

## 当前文件

- `jwt.go`：Signer / Verifier / key 解析 / claims context 主实现

## 当前实现事实

- 基于 RSA 密钥材料进行 RS256 签发与校验
- KID 由公钥派生，便于与 JWKS 发布链路对齐
- 同时提供通用 claims 的 context 注入/读取能力
- 该包是纯 JWT 基础设施，不直接绑定某个服务的 token 语义

## 边界约束

- 不在这里放认证 middleware；请求级认证应留给 `security/authn`
- 不在这里放业务 claims 模型、组织/项目权限解释或登录流程编排
- 不在这里发布 JWKS；公钥分发属于下游服务侧组件（如 `servora-iam/pkg/jwks`）

## 常见反模式

- 把业务 claims 结构直接写死在 `security/jwt`
- 在签发/验签工具里加入 HTTP 头读取或 transport 依赖
- 为了临时兼容而引入弱化校验的快捷路径

## 测试与使用

```bash
go test ./security/jwt/...
```

## 维护提示

- 若调整签名算法、KID 生成或 PEM 解析流程，需同步检查下游 JWKS 发布组件（如 `servora-iam/pkg/jwks`）与所有 token 使用方
- claims context helper 应保持通用，不要向某个业务 token 结构倾斜

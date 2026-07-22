# AGENTS.md - conformance/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-22 -->

## 当前定位

`conformance/` 保存跨语言实现共同消费的确定性 CRUD conformance vectors。它验证同一资源名合同在 Go 生成物与 TypeScript 生成物中的一致性，不是 mock、业务 fixture 或生成输出。

## 核心约束

- vector 必须是语言中立的 JSON，包含明确输入、预期 canonical 输出或预期失败。
- Go 与 TypeScript 测试必须消费同一份 vector；不得复制为各语言私有样例。
- 只记录稳定公开合同，不包含数据库 DSN、凭据、时间敏感值或机器路径。
- 修改 vector 时必须同时验证对应 Go/TypeScript consumer。

## 验证

在 `servora/` 执行：

```bash
go test ./cmd/protoc-gen-servora-crud
make web.test
```

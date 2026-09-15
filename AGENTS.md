# Servora

Proto contract 驱动的 Go 微服务框架；包含运行时、protoc 插件、公共 Proto、根 module 内生成 package 和前端共享包。

## 目录

- `api/protos/`：公共 Proto 与 annotation
- `api/gen/go/`：根 Go module 内的生成 package
- `cmd/`：CLI 与 `protoc-gen-servora-*`
- `security/`：通用 TLS primitive
- `obs/`：日志、追踪、指标、Audit
- `contrib/`：可选第三方 Client 与 capability Adapter
- `web/`：`@servora/proto-utils`

生成目录 `api/gen/go/`、`web/packages/proto-utils/src/gen/` 只由生成命令维护。

## 常用命令

```bash
just gen-fresh      # 清理并生成 Go Proto
just gen-ts         # 生成内建 Proto TypeScript
just lint-proto
just test
just test-all
just web-typecheck
just web-build
just tidy
```

删除或重命名 Proto 后使用 `just gen-fresh`；插件变更先运行 `just plugin`。CI parity 使用 `GOWORK=off`。

## Proto

- package 形如 `servora.<domain>.v1`，目录与 package 对齐。
- `go_package` 形如 `github.com/Servora-Kit/servora/api/gen/go/servora/<domain>/v1`。
- annotation 号段按命名空间从 `5xx00` 起；方法/消息使用 `+0`，服务/字段使用 `+1`。
- 方法级显式字段覆盖服务默认，未显式字段继承默认。

## 发布

Proto 或 Go 生成 package 变化时随根 module 的 `v0.x.y` 一起发布；既有 `api/gen/v*` tag 只保留为历史版本，不再新增。BSR 与 Go module 版本相互独立，前端包继续使用 `proto-utils/vx.y.z` tag。

提交格式：`type(scope): description`。

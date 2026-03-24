## Why

Kratos 的 `logging.Server` 中间件在打印请求日志时，会优先调用 proto message 的 `Redact() string` 方法输出脱敏后的内容。当前 servora 的 proto message 没有实现该接口，导致密码、token 等敏感字段以明文出现在日志中。

引入 `protoc-gen-redact`（来自 `buf.build/menta2k-org/redact`），通过 proto 注解声明哪些字段需要脱敏，自动生成 `Redact() string` 方法和 gRPC 响应装饰器。日志脱敏零运行时改动即可生效；gRPC 响应脱敏保留为未来可选能力。

## Non-goals

- 不自行编写或 fork `protoc-gen-redact` 插件——直接使用 `menta2k-org/redact` 三方包
- 不改动 Kratos logging 中间件或 `ChainBuilder`——现有 `logging.Server` 已内置 `Redacter` 接口消费
- 不在审计日志（`pkg/audit`）中做联动脱敏——审计事件不含 request body
- 不对 sayhello 示例服务或 audit 服务的 proto 做脱敏标注

## What Changes

- 根 `buf.yaml` 新增 dep：`buf.build/menta2k-org/redact`
- 根 `buf.go.gen.yaml` 新增 `protoc-gen-redact` 插件配置
- 根 `Makefile` 的 `plugin` target 新增 `protoc-gen-redact` 安装命令
- IAM 服务 proto 文件中为敏感字段添加 `(redact.v3.value)` 注解
- `api/gen/` 中生成 `*.pb.redact.go` 文件（含 `Redact() string` 方法和 `RegisterRedactedXxxServiceServer` 装饰器）
- `api/gen/go.mod` 新增运行时依赖 `github.com/menta2k/protoc-gen-redact/v3`

## Capabilities

### New Capabilities

- `proto-field-redact`: 通过 proto 注解 + protoc 插件实现字段级日志脱敏，生成的 `Redact()` 方法由 Kratos logging 中间件自动消费

### Modified Capabilities

（无）

## Impact

- **Proto 工具链**：新增一个 protoc 插件依赖和一个 Buf BSR 依赖
- **生成代码**：每个含注解的 proto 文件会多一个 `*.pb.redact.go`，所有 proto message 都会生成 `Redact()` 方法（无注解字段标记为 safe）
- **Go 依赖**：`api/gen/go.mod` 新增 `github.com/menta2k/protoc-gen-redact/v3` 运行时包
- **日志行为**：IAM 服务中被标注的字段在日志中不再以明文出现
- **gRPC 响应**：当前不使用 `RegisterRedactedXxxServiceServer`，但生成代码保留了该能力供未来启用

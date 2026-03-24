## Context

Kratos `logging.Server` 中间件在打印请求日志时通过 `extractArgs(req)` 获取请求内容的字符串表示。该函数优先检查 `Redacter` 接口（`Redact() string`），如有则使用脱敏后的输出；否则回退到 `String()` 或 `%+v`，暴露全部字段明文。

当前 servora 的 proto message 没有实现 `Redacter` 接口，IAM 服务中的密码、token、client secret 等敏感字段会以明文出现在日志中。

`protoc-gen-redact`（`buf.build/menta2k-org/redact`）是一个 protoc 插件，通过 proto 字段注解生成 `Redact() string` 方法，签名恰好匹配 Kratos 的 `Redacter` 接口。

```
                 Proto 编译链路
                 ============

  *.proto                    buf generate
  ├─ import redact.proto     ──────────────►  *.pb.redact.go
  ├─ field annotations                        ├─ Redact() string  ◄── Kratos logging 消费
  │  (redact.v3.value)                        └─ RegisterRedactedXxxServiceServer (保留)
  │
  └─ (其他 import)           ──────────────►  *.pb.go, *_grpc.pb.go, etc.
```

## Goals / Non-Goals

**Goals:**

- 在 IAM 服务的日志输出中，密码、token、client secret 等敏感字段不以明文出现
- 将 `protoc-gen-redact` 纳入标准工具链，后续新增 proto 时可直接使用注解
- 保留生成的 gRPC 响应脱敏装饰器代码，供未来启用

**Non-Goals:**

- 不改动 Kratos logging 中间件或 `ChainBuilder`
- 不为 sayhello、audit 服务添加脱敏注解
- 不启用 `RegisterRedactedXxxServiceServer`（gRPC 响应脱敏）
- 不 fork 或自定义 `protoc-gen-redact` 插件

## Decisions

### D1: 直接使用 `menta2k-org/redact` 而非 fork

**选择**：直接依赖 `buf.build/menta2k-org/redact` + `github.com/menta2k/protoc-gen-redact/v3`

**备选方案**：
- Fork 后改名 `protoc-gen-servora-redact`，统一到 `servora.redact.v1` 命名空间
- 用 protobuf 原生 `debug_redact` option + 自定义逻辑

**理由**：短期效果完全相同，fork 多花 1-2 天无额外收益。长期如需 BSR 统一发布或审计联动，再考虑 fork。

### D2: 脱敏策略——按字段语义选择 CLEAR 或 MASK

| 字段类型 | 策略 | 注解值 | 理由 |
|---------|------|--------|------|
| 密码类（password, password_confirm, current_password, new_password, new_password_confirm） | CLEAR | `(redact.v3.value).string = ""` | 密码无需保留任何痕迹 |
| Token 类（access_token, refresh_token, cap_token） | CLEAR | `(redact.v3.value).string = ""` | token 为高敏感凭证 |
| Client Secret（client_secret） | CLEAR | `(redact.v3.value).string = ""` | 同 token |
| 验证 token（VerifyEmail.token, ResetPassword.token） | CLEAR | `(redact.v3.value).string = ""` | 一次性凭证 |
| Email | MASK | `(redact.v3.value).string = "***@***.***"` | 保留"有 email"的上下文信息 |
| Phone | MASK | `(redact.v3.value).string = "***-****-****"` | 保留"有 phone"的上下文信息 |

### D3: Buf dep 放在根级别

根 `buf.yaml` 是 v2 workspace 模式，所有 deps 统一在根级别声明。`menta2k-org/redact` 作为跨 module 可用的 proto 依赖，放在根级别与 googleapis、kratos/apis 等并列。

### D4: protoc-gen-redact 安装位置

加到根 `Makefile` 的 `plugin` target，与 `protoc-gen-validate` 等外部 protoc 插件放在一起（在 servora 自定义插件之前）。

### D5: 注解仅添加到 IAM authn / application / user proto

IAM 服务包含三类敏感字段：
- `servora/authn/service/v1/authn.proto`：密码、token、email
- `servora/application/service/v1/application.proto`：client_secret
- `servora/user/service/v1/user.proto`：email、phone、password

其他 proto 文件（conf、pagination、organization、project 等）不含敏感字段，不加注解。

## Risks / Trade-offs

- **[R1] 三方依赖风险**：`menta2k-org/redact` 是 0-star 项目，可能停更 → 核心逻辑简单稳定，代码生成后不依赖上游更新；如停更可 fork
- **[R2] 生成代码膨胀**：每个 proto 文件多一个 `*.pb.redact.go`，包含所有 message 的 `Redact()` 方法（即使无注解） → 这是生成代码，不影响可维护性
- **[R3] 运行时依赖引入**：`api/gen/go.mod` 需加 `github.com/menta2k/protoc-gen-redact/v3`，即使只用日志脱敏也会引入 gRPC 装饰器相关的类型 → 编译期依赖，不影响运行时性能
- **[R4] 注解遗漏**：新增敏感字段时可能忘记加注解 → 建议在 PR review 中检查，未来可考虑 lint 规则

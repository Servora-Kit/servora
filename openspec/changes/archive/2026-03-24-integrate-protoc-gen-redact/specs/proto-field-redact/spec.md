## ADDED Requirements

### Requirement: Tool chain includes protoc-gen-redact

`make init`（通过 `make plugin`）SHALL 安装 `protoc-gen-redact` 二进制，使其可用于 `buf generate`。

#### Scenario: Fresh init installs redact plugin
- **WHEN** 开发者执行 `make init`
- **THEN** `protoc-gen-redact` 二进制存在于 `$GOPATH/bin/` 中

### Requirement: Buf workspace declares redact dependency

根 `buf.yaml` SHALL 在 `deps` 列表中包含 `buf.build/menta2k-org/redact`，使所有 proto module 可 import `redact/v3/redact.proto`。

#### Scenario: buf dep update succeeds
- **WHEN** 执行 `buf dep update`
- **THEN** `buf.lock` 中出现 `buf.build/menta2k-org/redact` 条目

### Requirement: Code generation produces redact files

根 `buf.go.gen.yaml` SHALL 包含 `protoc-gen-redact` 插件配置，输出到 `api/gen/go`，使用 `paths=source_relative`。

#### Scenario: make api generates redact files
- **WHEN** 执行 `make api`（或 `make api-go`）
- **THEN** 每个含 redact 注解的 proto 文件在 `api/gen/go/` 对应目录下生成 `*.pb.redact.go` 文件

#### Scenario: Generated files contain Redact method
- **WHEN** 查看生成的 `*.pb.redact.go`
- **THEN** 每个 proto message 均有 `Redact() string` 方法，被标注的字段执行脱敏替换

### Requirement: Password fields are cleared in logs

IAM 服务中所有密码类字段 SHALL 标注 `(redact.v3.value).string = ""`，使其在日志中显示为空字符串。

涉及字段：
- `SignupByEmailRequest.password`
- `SignupByEmailRequest.password_confirm`
- `LoginByEmailPasswordRequest.password`
- `ChangePasswordRequest.current_password`
- `ChangePasswordRequest.new_password`
- `ChangePasswordRequest.new_password_confirm`
- `ResetPasswordRequest.new_password`
- `ResetPasswordRequest.new_password_confirm`
- `CreateUserRequest.password`

#### Scenario: Password not in log output
- **WHEN** 客户端发送包含密码的请求（如 `LoginByEmailPassword`）
- **THEN** Kratos logging 中间件调用 `req.Redact()`，日志中密码字段显示为空

### Requirement: Token fields are cleared in logs

IAM 服务中所有 token 类字段 SHALL 标注 `(redact.v3.value).string = ""`。

涉及字段：
- `LoginByEmailPasswordResponse.access_token`
- `LoginByEmailPasswordResponse.refresh_token`
- `RefreshTokenRequest.refresh_token`
- `RefreshTokenResponse.access_token`
- `RefreshTokenResponse.refresh_token`
- `LogoutRequest.refresh_token`
- `SignupByEmailRequest.cap_token`
- `VerifyEmailRequest.token`
- `ResetPasswordRequest.token`

#### Scenario: Token not in log output
- **WHEN** 服务返回包含 access_token 的响应
- **THEN** 日志中 token 字段显示为空

### Requirement: Client secret fields are cleared in logs

应用服务中 client_secret 字段 SHALL 标注 `(redact.v3.value).string = ""`。

涉及字段：
- `CreateApplicationResponse.client_secret`
- `RegenerateClientSecretResponse.client_secret`

#### Scenario: Client secret not in log output
- **WHEN** 创建应用或重新生成 client secret
- **THEN** 日志中 client_secret 字段显示为空

### Requirement: Email fields are masked in logs

IAM 服务中 email 字段 SHALL 标注 `(redact.v3.value).string = "***@***.***"`，保留"有 email"的语义但不暴露具体值。

涉及字段：
- `SignupByEmailRequest.email`
- `SignupByEmailResponse.email`
- `LoginByEmailPasswordRequest.email`
- `RequestEmailVerificationRequest.email`
- `RequestPasswordResetRequest.email`
- `User.email`

#### Scenario: Email masked in log output
- **WHEN** 请求包含 email 字段
- **THEN** 日志中 email 显示为 `***@***.***`

### Requirement: Phone fields are masked in logs

IAM 服务中 phone 字段 SHALL 标注 `(redact.v3.value).string = "***-****-****"`。

涉及字段：
- `User.phone`

#### Scenario: Phone masked in log output
- **WHEN** 请求或响应包含 phone 字段
- **THEN** 日志中 phone 显示为 `***-****-****`

### Requirement: Runtime dependency is available

`api/gen/go.mod` SHALL 包含 `github.com/menta2k/protoc-gen-redact/v3` 依赖，使生成的 `*.pb.redact.go` 可编译通过。

#### Scenario: Generated code compiles
- **WHEN** 执行 `go build ./api/gen/...`
- **THEN** 编译成功，无 import 错误

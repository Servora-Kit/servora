## 1. 工具链配置

- [x] 1.1 根 `buf.yaml` deps 新增 `buf.build/menta2k-org/redact`
- [x] 1.2 执行 `buf dep update` 更新 `buf.lock`
- [x] 1.3 根 `buf.go.gen.yaml` plugins 新增 `protoc-gen-redact`（out: api/gen/go, opt: paths=source_relative, lang=go）
- [x] 1.4 根 `Makefile` plugin target 新增 `go install github.com/menta2k/protoc-gen-redact/v3/cmd/protoc-gen-redact@latest`

## 2. Proto 注解

- [x] 2.1 `servora/authn/service/v1/authn.proto`：import redact.proto，为密码、token、email 字段添加脱敏注解
- [x] 2.2 `servora/application/service/v1/application.proto`：import redact.proto，为 client_secret 字段添加脱敏注解
- [x] 2.3 `servora/user/service/v1/user.proto`：import redact.proto，为 email、phone、password 字段添加脱敏注解

## 3. 代码生成与依赖

- [x] 3.1 执行 `make plugin` 安装 protoc-gen-redact
- [x] 3.2 执行 `make api`（`buf generate`）生成 `*.pb.redact.go` 文件
- [x] 3.3 `api/gen/go.mod` 执行 `go get github.com/menta2k/protoc-gen-redact/v3` 添加运行时依赖
- [x] 3.4 执行 `go mod tidy` 清理依赖

## 4. 验证

- [x] 4.1 确认 `api/gen/go/` 下生成了 `*.pb.redact.go` 文件
- [x] 4.2 确认生成的代码编译通过（`go build ./api/gen/...`）
- [x] 4.3 确认标注字段的 `Redact()` 方法中执行了脱敏替换

## 新增需求

## 修改需求

### 需求:必须创建 api/gen/go.mod 模块

系统必须在 `api/gen/` 目录创建独立的 `go.mod` 文件，将生成的 proto 代码作为独立模块管理。

#### 场景:go.mod 位置

- **当** 创建 api/gen 模块
- **那么** `go.mod` 必须位于 `api/gen/go.mod`（而不是 `api/gen/go/go.mod`）
- **那么** 模块路径必须为 `module github.com/Servora-Kit/servora/api/gen`

#### 场景:避免被 buf generate 删除

- **当** 执行 `buf generate` 并配置 `clean: true`
- **那么** `api/gen/go.mod` 必须不被删除
- **那么** 只有 `api/gen/go/` 子目录被清理

#### 场景:包含必要的依赖

- **当** 创建 `api/gen/go.mod`
- **那么** 必须包含以下依赖：
  - `google.golang.org/protobuf` (protobuf 运行时)
  - `google.golang.org/grpc` (gRPC 运行时)
  - `github.com/go-kratos/kratos/v2` (Kratos 框架)
  - `github.com/envoyproxy/protoc-gen-validate` (验证器)

#### 场景:import 路径保持不变

- **当** 生成代码位于 `api/gen/go/`
- **那么** import 路径必须为 `github.com/Servora-Kit/servora/api/gen/go/<path>`
- **那么** 现有代码的 import 语句需要更新为新的组织路径

## 移除需求

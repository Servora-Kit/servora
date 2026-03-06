## 为什么

项目从个人账户迁移到组织账户，以更好地反映项目的协作性质和长期维护计划。当前所有代码、proto 文件和配置中硬编码的 `github.com/horonlee/servora` 路径需要统一更新为 `github.com/Servora-Kit/servora`，确保 Go 模块导入、proto 生成和文档链接的一致性。

## 变更内容

- 更新所有 proto 文件中的 `go_package` 选项，从 `github.com/horonlee/servora` 改为 `github.com/Servora-Kit/servora`
- 更新所有 Go 模块文件（go.mod）中的 module 路径和 replace 指令
- 更新所有 Go 源代码中的 import 路径
- 更新 OpenAPI 配置文件（buf.openapi.gen.yaml）中的 contact URL
- 更新文档（README.md、AGENTS.md 等）中的 GitHub 仓库链接
- 重新生成所有 proto 相关的 Go 代码（api/gen/go/）
- 重新生成 Wire 依赖注入代码（wire_gen.go）

## 功能 (Capabilities)

### 新增功能
- `github-org-migration`: 支持从个人账户到组织账户的 GitHub 仓库迁移，包括所有代码引用的自动更新

### 修改功能
- `proto-go-package-declaration`: 更新 proto 文件的 go_package 声明以使用新的组织路径
- `go-workspace-config`: 更新 Go workspace 和模块配置以使用新的组织路径
- `api-gen-module`: 更新 API 生成模块的路径引用

## 影响

- **受影响文件**: 约 125 个文件，包括：
  - 所有 proto 文件（*.proto）
  - 所有 Go 模块文件（go.mod）
  - 所有生成的 Go 代码（api/gen/go/**/*.pb.go）
  - 所有手写的 Go 源代码（import 语句）
  - OpenAPI 配置文件
  - 文档文件（README.md、AGENTS.md 等）
  - OpenSpec 归档文件

- **受影响系统**:
  - Go 模块系统（需要更新所有 import 路径）
  - Proto 代码生成流程（需要重新生成）
  - Wire 依赖注入（需要重新生成）
  - 文档和链接引用

- **破坏性变更**:
  - **BREAKING**: 所有外部依赖此项目的代码需要更新 import 路径
  - **BREAKING**: 已发布的 Go 模块版本将指向旧路径，新版本需要使用新路径

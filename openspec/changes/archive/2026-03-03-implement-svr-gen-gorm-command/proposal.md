## Why

当前每个使用 GORM GEN 的微服务都需要维护自己的 `cmd/gen/gorm-gen.go` 文件，这些文件的逻辑几乎完全相同（连接数据库、配置生成器、执行生成），造成了大量重复代码。通过实现中心化的 `svr gen gorm` 命令，可以消除这些重复，统一生成逻辑，并简化服务的维护工作。

## What Changes

1. 在 `cmd/svr` 中新增 `svr gen gorm <服务名...>` 命令，支持多个服务名批量生成
2. 无参数执行 `svr gen gorm` 时进入交互式服务选择（huh）
3. 实现配置加载模块，复用 `pkg/bootstrap/config/loader.go` 读取服务配置
4. 实现 GORM GEN 生成器封装，支持 MySQL/PostgreSQL/SQLite
5. 生成 DAO 和 PO 代码到 `app/<服务名>/service/internal/data/gorm/{dao,po}`
6. 更新 `app.mk` 中的 `gen.gorm` 目标，调用新的中心化命令
7. 提供 `--dry-run` 选项用于预览生成路径
8. 使用 `charmbracelet/lipgloss` 美化 CLI 输出，参考 Ech0 项目设计
9. 支持多服务执行时“失败不中断”，最终汇总成功/失败数量和失败详情
10. 提供清晰的错误提示（服务不存在、配置缺失、数据库连接失败等场景）

## Capabilities

### New Capabilities

- `svr-gen-gorm-command`: 中心化的 GORM GEN 代码生成命令，支持单个或多个服务批量生成 DAO/PO 代码
- `service-config-loader`: 服务配置加载模块，用于从服务的 config.yaml 中提取数据库配置
- `gorm-generator`: GORM GEN 生成器封装，处理数据库连接和代码生成逻辑
- `cli-ux-enhancement`: CLI 用户体验增强，使用 lipgloss 美化输出，并在无参数时使用 huh 交互选择服务

### Modified Capabilities

- `app-mk-gen-gorm`: 更新 `app.mk` 中的 `gen.gorm` 目标，从本地脚本改为调用中心化命令

## Impact

**受影响的代码**：
- `cmd/svr/` - 新增命令行工具代码
- `app.mk` - 更新 `gen.gorm` 目标
- `app/*/service/cmd/gen/gorm-gen.go` - 可选择性移除（向后兼容）

**受影响的工作流**：
- 开发者使用 `make gen.gorm` 的体验保持不变
- 新增 `svr gen gorm <服务名>` 作为直接调用方式
- 首次生成时需要确保数据库可访问

**依赖项**：
- 依赖 `pkg/bootstrap/config/loader.go`（已存在）
- 依赖 `gorm.io/gen`（已存在）
- 依赖 `github.com/spf13/cobra`（需要添加）
- 依赖 `github.com/charmbracelet/lipgloss`（需要添加，用于美化输出）
- 依赖 `github.com/charmbracelet/huh`（用于无参数交互式服务选择）

**破坏性变更**：
- 无破坏性变更，完全向后兼容

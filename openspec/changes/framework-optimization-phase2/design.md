## 上下文

当前框架已实现双分支策略（main = 框架，example = 完整项目），但开发体验和规范化仍有不足。本设计旨在优化 Git Hooks 管理、文档结构和开发流程，提升框架的可维护性和新开发者的上手体验。

关键约束：
- 保持 pre-commit hook 快速执行（~1s），不影响开发体验
- Git Hooks 同步机制必须自动化，减少手动操作
- README 差异化策略不能影响分支合并流程
- .env 文件管理必须平衡示例性和安全性

## 目标 / 非目标

**目标：**
- 通用化 Git Hooks 规则，支持任意服务名称
- 自动化 Hooks 同步机制，确保分支切换后 hooks 始终最新
- 差异化 README 内容，main 展示框架能力，example 展示完整项目
- 规范化 .env 文件管理，提供清晰的使用指南

**非目标：**
- 实现 CI/CD 流程（本次仅提供设计文档）
- 添加更多 lint 检查（如 golangci-lint，保持 pre-commit 快速）
- 修改现有的 commit message 格式（仅优化 scope 规则）

## 决策

### 决策 1: Git Hooks Scope 通用化

**选择**：使用通用类别（pkg、cmd、app、example、openspec、infra）替代具体服务名（servora、sayhello）

**理由**：
- 具体服务名硬编码导致每次添加新服务都需要修改 hooks
- 通用类别更符合框架的分层架构（框架代码 vs 应用代码）
- 与双分支策略一致：main 分支只包含 pkg/cmd，example 分支包含 app

**替代方案**：
- 方案 A：保持具体服务名，每次添加服务时更新 hooks（被拒绝：维护成本高）
- 方案 B：完全移除 scope 限制（被拒绝：失去分类价值）

### 决策 2: Git Hooks 同步机制 - 符号链接 + post-merge Hook

**选择**：结合符号链接（方案 B）和 post-merge hook（方案 A）

**理由**：
- 符号链接：开发时修改 scripts/git-hooks/ 立即生效，无需重新安装
- post-merge hook：分支切换和合并后自动同步，确保 hooks 版本正确
- 两者结合：既保证开发便利性，又保证分支切换后的一致性

**实现细节**：
- install-hooks.sh 支持 --symlink 参数，默认使用符号链接
- post-merge hook 检测 scripts/git-hooks/ 变更后自动执行 install-hooks.sh
- 首次克隆仓库时需要手动运行 scripts/install-hooks.sh

**替代方案**：
- 方案 A（仅 post-merge）：每次修改 hooks 都需要提交才能生效（被拒绝：开发不便）
- 方案 B（仅符号链接）：分支切换后可能使用错误版本的 hooks（被拒绝：不够安全）

### 决策 3: pre-commit Hook 增强 - 仅分支检查 + gofmt

**选择**：pre-commit 只执行分支检查和 gofmt，不添加其他 lint

**理由**：
- 分支检查：防止在 main 分支误提交 app/ 目录（~100ms）
- gofmt：确保代码格式一致，执行速度快（~500ms）
- 总执行时间：~1s，符合快速执行要求

**不包含的检查**：
- golangci-lint：执行时间较长（5-10s），影响开发体验
- buf lint：proto lint 输出较长，且当前不需要
- 大文件检查：当前项目规模较小，暂不需要

**替代方案**：
- 添加 golangci-lint（被拒绝：执行时间过长）
- 添加 buf lint（被拒绝：用户明确表示不需要）

### 决策 4: README 差异化策略 - 方案 3（条件内容）

**选择**：main 和 example 分支维护不同的 README.md 内容

**理由**：
- main 分支：聚焦框架能力、架构设计、扩展方式
- example 分支：包含完整项目的运行指南、示例服务说明
- 分支合并时需要手动处理 README 冲突，但这是可接受的

**实现细节**：
- main 分支 README：框架概述、技术栈、架构设计、快速开始（引导到 example 分支）
- example 分支 README：完整运行指南、环境准备、服务启动、API 测试、示例服务说明
- 合并策略：从 example 到 main 时，保留 main 的 README；从 main 到 example 时，保留 example 的 README

**替代方案**：
- 方案 1（单一 README）：无法同时满足框架说明和运行指南（被拒绝）
- 方案 2（README + QUICKSTART）：文件过多，维护成本高（被拒绝）

### 决策 5: .env 文件管理 - 详细注释 + .env.local

**选择**：example 分支的 .env 包含详细警告注释和示例值，真实配置使用 .env.local

**理由**：
- .env 作为示例文件提交，包含占位符和详细使用说明
- .env.local 存储真实敏感信息，被 .gitignore 忽略
- 注释风格：详细说明（而非简短提示），帮助新开发者理解最佳实践

**实现细节**：
- .env 文件顶部包含多行注释，说明：
  - 禁止提交真实敏感信息
  - 应该创建 .env.local 存储本地配置
  - .env.local 已被 .gitignore 忽略
- 所有敏感字段使用占位符（如 "your_password_here"）
- .gitignore 添加 .env.local 条目

**替代方案**：
- 简短注释风格（被拒绝：用户要求详细说明）
- 不提交 .env 文件（被拒绝：失去示例价值）

## 风险 / 权衡

### 风险 1: 符号链接在 Windows 上的兼容性

**风险**：Windows 系统创建符号链接需要管理员权限或开发者模式

**缓解措施**：
- install-hooks.sh 检测符号链接创建失败时，自动回退到文件复制模式
- 文档中说明 Windows 用户需要启用开发者模式或使用管理员权限

### 风险 2: README 合并冲突

**风险**：main 和 example 分支的 README 内容不同，合并时会产生冲突

**缓解措施**：
- 在 CLAUDE.md 中明确说明 README 合并策略
- 提供清晰的冲突解决指南：保留目标分支的 README 内容

### 风险 3: post-merge Hook 可能被跳过

**风险**：开发者使用 --no-verify 跳过 post-merge hook

**缓解措施**：
- 在文档中强调不要跳过 hooks
- post-merge hook 执行失败时显示清晰的错误信息

### 风险 4: gofmt 检查可能拒绝合法提交

**风险**：某些自动生成的代码可能不符合 gofmt 格式

**缓解措施**：
- pre-commit hook 只检查 staged 的 .go 文件，不检查所有文件
- 对于自动生成的代码（如 wire_gen.go），可以在 pre-commit hook 中添加排除规则

## CI/CD 设计（未来实现）

### main 分支 CI/CD

**触发条件**：push 到 main 分支

**流程**：
1. 代码检查：golangci-lint、buf lint
2. 单元测试：go test ./pkg/... ./cmd/...
3. 构建验证：go build ./cmd/svr

**不包含**：
- 服务启动测试（main 分支没有完整服务）
- 集成测试（main 分支没有数据库配置）

### example 分支 CI/CD

**触发条件**：push 到 example 分支

**流程**：
1. 代码检查：golangci-lint、buf lint
2. 单元测试：go test ./...
3. 集成测试：使用 testcontainers 启动数据库，运行集成测试
4. 服务启动测试：启动 servora 和 sayhello 服务，执行健康检查

**未来扩展**：
- 添加 API 测试（使用 Postman/Newman）
- 添加性能测试（使用 k6）

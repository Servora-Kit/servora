# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 双分支策略

**重要**：本仓库采用双分支架构，AI 开发时必须遵循以下规则：

### 分支说明

- **main 分支**：纯框架代码，用于 Go module 发布
  - 包含：`pkg/`、`cmd/svr/`、`api/protos/`、`templates/`、文档
  - 不包含：服务实现（`app/`）、部署配置（`manifests/`、`docker-compose.yaml`）

- **example 分支**：完整示例项目
  - 包含：框架代码 + 示例服务（servora、sayhello）+ 部署配置
  - 用于：开发、测试、演示

### templates vs manifests

**重要区别**：这两个目录服务于不同的目的，不应该相互同步。

- **templates/**（main 分支）
  - 通用的、框架级别的部署模板
  - 给使用框架的人作为参考
  - 高度参数化，不包含具体的服务配置
  - 修改场景：改进框架级别的部署最佳实践

- **manifests/**（example 分支）
  - 具体的、可运行的部署配置
  - 基于 templates 创建的实例
  - 包含具体的服务名、端口、环境变量等
  - 修改场景：调整 example 项目的部署配置

**工作流**：
- 改进部署模板 → 在 main 分支修改 `templates/`
- 调整示例配置 → 在 example 分支修改 `manifests/`
- 不需要从 example 同步 manifests 到 main

### AI 开发规则

1. **始终在 example 分支开发**
   ```bash
   git checkout example
   ```

2. **不要在 main 分支直接开发**
   - main 分支缺少服务代码和部署配置，无法运行和测试
   - Git hooks 会阻止在 main 分支提交服务代码

3. **框架提交需要同步到 main**
   - 如果修改了 `pkg/` 或 `cmd/`，提交后需要同步到 main 分支
   - 使用 `git cherry-pick` 同步（见下文）

## 提交消息格式要求

**强制规范**：所有提交必须遵循以下格式（git hooks 会自动验证）：

```
type(scope): description
```

### 允许的 type

- `feat`: 新功能
- `fix`: Bug 修复
- `refactor`: 重构
- `docs`: 文档
- `test`: 测试
- `chore`: 构建/工具

### 允许的 scope

- `pkg`: 框架核心代码（需要同步到 main）
- `cmd`: CLI 工具（需要同步到 main）
- `app`: 应用服务（仅 example 分支）
- `example`: 示例配置（仅 example 分支）
- `openspec`: OpenSpec 变更管理（需要同步到 main）
- `infra`: 基础设施/部署（需要同步到 main）

### 提交消息示例

```bash
# 正确 ✓
feat(pkg): add authentication middleware
fix(cmd): correct flag parsing in svr command
feat(app): add user registration endpoint
docs(example): update deployment guide
chore(infra): update kubernetes deployment config

# 错误 ✗
add authentication middleware          # 缺少 type 和 scope
feat: add middleware                   # 缺少 scope
feat(auth): add middleware             # scope 不在允许列表中
Add authentication middleware          # 首字母大写
feat(pkg): Add authentication.         # 描述首字母大写或有句号
```

### 提交最佳实践

1. **保持提交小而专注**：一个提交只做一件事
2. **避免混合提交**：不要在同一个提交中同时修改框架和服务代码
3. **使用清晰的描述**：描述"做了什么"，而不是"怎么做的"
4. **遵循格式**：git hooks 会自动验证，不符合格式的提交会被拒绝

### 规范的灵活性

**重要**：Git hooks 的规范是可以调整的，不是一成不变的。

当你（AI）在工作中发现：
- 现有的 `type` 无法准确描述你的提交类型
- 现有的 `scope` 无法覆盖你要修改的代码范围
- 需要使用更合适的提交分类

**你应该**：
1. **主动询问用户**：说明为什么需要新的 type 或 scope
2. **提供建议**：建议添加什么内容以及它的用途
3. **等待确认**：用户同意后，更新 git hooks 和 CLAUDE.md

**示例场景**：
- 需要 `perf` type 来标记性能优化提交
- 需要 `api` scope 来专门标记 API 相关的更改
- 需要 `security` type 来标记安全修复

**不要**：
- 不要默默使用不在列表中的 type/scope（会被 hooks 拒绝）
- 不要强行将提交归类到不合适的 type/scope
- 不要在没有询问的情况下修改 hooks

规范应该服务于开发，而不是限制开发。保持沟通，让规范随项目演进。

## 同步框架提交到 Main

当你修改了框架代码（`pkg/` 或 `cmd/`）后，需要将这些提交同步到 main 分支：

### 步骤 1：识别框架提交

框架相关的提交（scope 为 `pkg` 或 `cmd`）需要同步：

```bash
# 查看最近的框架提交
git log --oneline --grep="(pkg):" --grep="(cmd):" | head -5
```

### 步骤 2：同步到 main

```bash
# 切换到 main 分支
git checkout main

# Cherry-pick 框架提交
git cherry-pick <commit-hash>

# 切换回 example 分支
git checkout example
```

### 步骤 3：处理冲突（如果有）

```bash
# 如果有冲突
git status                    # 查看冲突文件
# 解决冲突...
git add <resolved-files>
git cherry-pick --continue
```

## Git Hooks 说明

本仓库使用 git hooks 强制执行规范：

### commit-msg hook

验证提交消息格式：
- 检查 `type(scope): description` 格式
- 验证 type 和 scope 在允许列表中
- 允许 Merge 和 Revert 提交

### pre-commit hook

防止在 main 分支提交服务代码：
- 检查是否在 main 分支
- 阻止提交 `app/` 目录的文件
- 阻止提交 `manifests/`、`docker-compose.yaml` 等部署配置
- 执行 gofmt 格式检查（仅检查 staged 的 .go 文件）

### post-merge hook

自动同步 git hooks：
- 检测 `scripts/git-hooks/` 目录的变更
- 合并或拉取后自动执行 `scripts/install-hooks.sh`
- 确保 hooks 始终保持最新版本

### 安装 hooks

```bash
bash scripts/install-hooks.sh
```

默认使用符号链接安装，修改 `scripts/git-hooks/` 中的 hooks 会立即生效。

**重要**：不要使用 `--no-verify` 跳过 hooks 验证。

## README 合并策略

**重要**：main 和 example 分支的 README.md 内容不同，合并时会产生冲突。

### 合并规则

- **从 example 合并到 main**：保留 main 分支的 README.md（框架说明）
- **从 main 合并到 example**：保留 example 分支的 README.md（完整项目说明）

### 处理冲突

```bash
# 如果合并时 README.md 冲突
git checkout --ours README.md    # 保留当前分支的 README
# 或
git checkout --theirs README.md  # 使用合并来源分支的 README

git add README.md
git commit
```

**原则**：始终保留目标分支（你当前所在分支）的 README 内容。

## Documentation Structure

This repository uses a hierarchical AGENTS.md documentation system. **Always read the relevant AGENTS.md files** to understand the codebase structure, conventions, and workflows.

### Available AGENTS.md Files

- **Root**: `AGENTS.md` - Project overview, top-level structure, and common commands
- **API Layer**: `api/AGENTS.md` - Proto organization and code generation rules
- **Shared Protos**: `api/protos/AGENTS.md` - Shared proto module details
- **CLI Tool**: `cmd/svr/AGENTS.md` - CLI tool usage and implementation
- **Services**: Check individual service directories for service-specific documentation

### How to Use AGENTS.md Files

1. Start with the root `AGENTS.md` for project overview
2. Navigate to subdirectory AGENTS.md files for specific component details
3. AGENTS.md files contain:
   - Current structure and facts
   - Common commands
   - Development conventions
   - Maintenance tips

### Quick Reference

For detailed information, read the AGENTS.md files. Key entry points:
- Project setup and commands: `AGENTS.md`
- Proto and API generation: `api/AGENTS.md`
- CLI tool usage: `cmd/svr/AGENTS.md`

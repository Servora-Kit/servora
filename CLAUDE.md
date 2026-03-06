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
- `servora`: servora 服务（仅 example 分支）
- `sayhello`: sayhello 服务（仅 example 分支）
- `example`: 示例配置（仅 example 分支）

### 提交消息示例

```bash
# 正确 ✓
feat(pkg): add authentication middleware
fix(cmd): correct flag parsing in svr command
refactor(servora): improve user service structure
docs(example): update deployment guide

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

### 安装 hooks

```bash
bash scripts/install-hooks.sh
```

**重要**：不要使用 `--no-verify` 跳过 hooks 验证。

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

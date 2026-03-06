## 为什么

当前框架已完成双分支策略的基础实施（main = 框架，example = 完整项目），但仍存在开发体验和规范化方面的不足：Git Hooks 规则过于具体（硬编码服务名）、README 未反映双分支策略、.env 文件缺少使用说明、Hooks 同步机制不够自动化。这些问题影响了框架的可维护性和新开发者的上手体验。

## 变更内容

1. **README.md 双分支差异化**：main 分支展示框架能力和架构，example 分支展示完整项目的运行和开发指南
2. **Git Hooks 通用化**：将 scope 从具体服务名（servora/sayhello）改为通用类别（app），pre-commit 检查从具体服务路径改为通用 `app/` 路径
3. **pre-commit Hook 增强**：添加 gofmt 格式检查，保持快速执行（~1s）
4. **Git Hooks 同步机制**：结合符号链接（方案B）和 post-merge hook（方案A），确保 hooks 在分支切换和合并后自动同步
5. **.env 文件文档化**：在 example 分支的 .env 中添加详细的警告注释，说明敏感信息管理和 .env.local 的使用
6. **CI/CD 设计文档**：为未来实现提供双分支 CI/CD 架构设计（本次不实现）

## 功能 (Capabilities)

### 新增功能
- `git-hooks-sync`: Git Hooks 自动同步机制，确保 scripts/git-hooks/ 与 .git/hooks/ 保持一致
- `dual-branch-readme`: 双分支差异化 README 策略，main 和 example 分支展示不同内容
- `env-file-guide`: .env 文件使用指南和最佳实践文档

### 修改功能
- `git-hooks-validation`: 现有的 commit-msg 和 pre-commit hooks，通用化 scope 和路径检查规则

## 影响

- **文档**：README.md（两个分支内容不同）、.env（example 分支）、.gitignore
- **Git Hooks**：scripts/git-hooks/commit-msg、scripts/git-hooks/pre-commit、scripts/git-hooks/post-merge（新增）、scripts/install-hooks.sh
- **开发流程**：分支切换后 hooks 自动同步，commit 时 scope 规则更灵活
- **CI/CD**：未来需要为 main 和 example 分支设计不同的 workflow（本次仅设计，不实现）

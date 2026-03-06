## 上下文

当前 servora 仓库是一个混合了框架代码和示例服务的单体仓库。随着项目从个人账号迁移到组织账号，我们需要明确框架的定位：它应该是一个可以被其他项目依赖的 Go module，而不是一个包含具体业务服务的脚手架项目。

**当前状态**：
- 根目录包含框架代码（pkg/、cmd/svr/）和示例服务（app/servora/、app/sayhello/）
- 部署配置（manifests/）混合了基础设施和服务特定配置
- 无法直接作为纯框架 Go module 发布
- 开发时需要完整环境来调试框架功能

**约束**：
- 早期开发阶段，需要频繁修改框架代码并在真实服务中验证
- 不能使用多仓库方案，因为会增加同步复杂度
- 必须保持 git 历史的连续性
- 需要支持 AI 协作开发，要求严格的提交规范

## 目标 / 非目标

**目标：**
- 将仓库改造为双分支架构：main（框架）+ example（示例）
- 建立 example → main 的反向工作流程，支持在完整环境中开发框架
- 通过 git hooks 强制执行提交规范，规范人类和 AI 的行为
- 提供辅助工具简化分支间的代码同步

**非目标：**
- 不创建独立的服务仓库（暂时保留在 example 分支）
- 不使用 git subrepo 或 submodule（增加复杂度）
- 不改变现有的构建系统和开发工具链
- 不影响已有的 OpenSpec 工作流程

## 决策

### 决策 1：使用分支而不是独立仓库

**选择**：在同一仓库中使用 main 和 example 两个分支

**理由**：
- ✅ 简化开发流程：无需跨仓库同步，可以在同一个 PR 中修改框架和示例
- ✅ 保持 git 历史：所有变更在同一个历史线中，方便追溯
- ✅ 降低复杂度：不需要管理多个仓库的版本依赖
- ✅ 灵活演进：未来可以轻松导出为独立仓库

**替代方案**：
- ❌ 多仓库方案：需要发布框架版本、更新依赖，迭代速度慢
- ❌ Monorepo + Go workspace：无法清晰分离框架和示例的发布边界

### 决策 2：example → main 反向工作流程

**选择**：主要开发在 example 分支，通过 cherry-pick 同步框架提交到 main

**理由**：
- ✅ 更自然：在完整环境中开发和测试，立即看到效果
- ✅ 更安全：框架修改在真实服务中验证后再同步到 main
- ✅ 更灵活：可以同时修改框架和服务，快速迭代

**替代方案**：
- ❌ main → example 正向流程：需要先在 main 开发，再切换到 example 测试，上下文切换频繁
- ❌ 双向同步：容易产生冲突，难以管理

**实现方式**：
- 使用严格的提交消息格式区分框架和服务提交
- 提供 git alias 快速查看需要同步的框架提交
- 使用 `git cherry-pick` 精确控制同步内容

### 决策 3：使用 git hooks 强制提交规范

**选择**：使用 commit-msg hook 验证提交消息格式

**理由**：
- ✅ 从一开始建立规范，避免后期清理历史
- ✅ 规范 AI 行为：AI 必须遵循格式才能提交
- ✅ 自动化：不依赖人工审查

**格式**：`type(scope): description`
- type: feat, fix, refactor, docs, test, chore
- scope: pkg, cmd, servora, sayhello, example
- 框架相关 scope (pkg, cmd) 的提交需要同步到 main

**替代方案**：
- ❌ 只用 git alias：依赖人工记忆，容易遗忘
- ❌ CI 检查：提交后才发现问题，需要修改历史

### 决策 4：templates/ 目录结构

**选择**：创建 templates/ 目录存放配置模板

**结构**：
```
templates/
├── k8s/              # K8s 部署模板
│   ├── base/         # 基础资源（namespace, rbac）
│   └── service/      # 服务部署模板
├── docker/           # Docker 配置模板
│   ├── Dockerfile
│   └── docker-compose.yaml
└── observability/    # 可观测性配置模板
    ├── grafana/
    ├── loki/
    ├── otel/
    └── prometheus/
```

**理由**：
- ✅ 清晰分离：框架提供模板，服务自行定制
- ✅ 可复用：新项目可以直接复制模板
- ✅ 可维护：模板更新不影响 example 分支的实际配置

## 风险 / 权衡

### 风险 1：分支同步冲突

**风险**：example 分支定期 merge main 时可能产生冲突

**缓解措施**：
- 框架提交保持小而专注，减少冲突面
- 定期同步（每天或每个功能完成后）
- 使用 cherry-pick 而不是 merge，避免不必要的合并提交

### 风险 2：提交历史混乱

**风险**：example 分支包含框架和服务的混合提交，历史不够清晰

**缓解措施**：
- 严格的提交消息格式，通过 scope 区分
- 避免混合提交（一个提交只改框架或只改服务）
- 提供 git alias 快速过滤框架提交

### 风险 3：git hooks 不被跟踪

**风险**：`.git/hooks/` 不会被 git 跟踪，新克隆的仓库没有 hooks

**缓解措施**：
- 将 hooks 脚本放在 `scripts/git-hooks/` 目录
- 提供 `scripts/install-hooks.sh` 安装脚本
- 在 README 和 DEVELOPMENT.md 中说明安装步骤

### 风险 4：AI 可能不遵循规范

**风险**：AI 可能尝试绕过 hooks 或使用错误的提交格式

**缓解措施**：
- 在 CLAUDE.md 中明确说明提交规范
- hooks 提供清晰的错误消息和示例
- 不允许使用 `--no-verify` 标志

## 迁移计划

### 阶段 1：准备（不影响现有开发）

1. 创建 `scripts/git-hooks/` 目录和 hooks 脚本
2. 创建 `scripts/install-hooks.sh` 安装脚本
3. 更新 DEVELOPMENT.md 文档

### 阶段 2：创建分支结构

1. 创建 example 分支（保存当前完整项目）
2. 在 main 分支创建 templates/ 目录
3. 从 manifests/ 提取配置模板到 templates/

### 阶段 3：清理 main 分支

1. 删除 app/ 目录
2. 删除 manifests/ 目录
3. 删除 docker-compose.yaml 和 docker-compose.dev.yaml
4. 删除 go.work 和 go.work.sum
5. 更新 README.md 说明分支用途

### 阶段 4：验证和文档

1. 在 example 分支测试完整开发流程
2. 验证 git hooks 正常工作
3. 更新 CLAUDE.md 说明工作流程
4. 创建 example 分支的 README.md

### 回滚策略

如果迁移出现问题：
- example 分支保留了完整的原始状态
- 可以直接切换回 example 分支继续开发
- main 分支的修改可以通过 git reset 回退

## Open Questions

1. **Go module 版本管理**：main 分支如何发版？使用语义化版本？
2. **共享 proto 的处理**：api/protos/ 中的共享 proto 是否需要独立发布？
3. **CI/CD 配置**：两个分支是否需要不同的 CI 流程？
4. **分支保护规则**：main 分支是否需要设置保护规则？

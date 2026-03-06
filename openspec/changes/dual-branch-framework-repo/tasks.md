## 1. 准备 Git Hooks 和脚本

- [x] 1.1 创建 scripts/git-hooks/ 目录
- [x] 1.2 创建 commit-msg hook 脚本验证提交消息格式
- [x] 1.3 创建 pre-commit hook 脚本防止在 main 分支提交服务代码
- [x] 1.4 创建 scripts/install-hooks.sh 安装脚本
- [x] 1.5 测试 hooks 脚本的验证逻辑

## 2. 创建 Templates 目录结构

- [x] 2.1 创建 templates/k8s/base/ 目录
- [x] 2.2 从 manifests/k8s/base/ 复制 namespace.yaml 到 templates/k8s/base/
- [x] 2.3 从 manifests/k8s/base/ 复制 rbac.yaml 到 templates/k8s/base/
- [x] 2.4 创建 templates/k8s/service/ 目录并添加服务部署模板
- [x] 2.5 创建 templates/docker/ 目录
- [x] 2.6 创建 Dockerfile 模板到 templates/docker/
- [x] 2.7 创建 docker-compose.yaml 模板到 templates/docker/
- [x] 2.8 创建 templates/observability/ 目录
- [x] 2.9 从 manifests/grafana/ 复制配置到 templates/observability/grafana/
- [x] 2.10 从 manifests/loki/ 复制配置到 templates/observability/loki/
- [x] 2.11 从 manifests/otel/ 复制配置到 templates/observability/otel/
- [x] 2.12 从 manifests/prometheus/ 复制配置到 templates/observability/prometheus/

## 3. 创建分支结构

- [x] 3.1 创建 example 分支（从当前 main 分支）
- [x] 3.2 验证 example 分支包含完整项目结构
- [x] 3.3 在 example 分支测试 docker-compose up 能否正常启动

## 4. 清理 Main 分支

- [x] 4.1 切换回 main 分支
- [x] 4.2 删除 app/ 目录
- [x] 4.3 删除 manifests/ 目录
- [x] 4.4 删除 docker-compose.yaml
- [x] 4.5 删除 docker-compose.dev.yaml
- [x] 4.6 删除 go.work
- [x] 4.7 删除 go.work.sum
- [x] 4.8 删除 Dockerfile.air（如果存在）
- [x] 4.9 验证 main 分支只包含框架代码

## 5. 更新 Main 分支 README

- [x] 5.1 更新 README.md 说明这是框架仓库
- [x] 5.2 添加 example 分支的引导链接
- [x] 5.3 添加框架安装说明（go get）
- [x] 5.4 添加 git hooks 安装说明
- [x] 5.5 说明双分支策略和工作流程

## 6. 创建 Example 分支 README

- [x] 6.1 切换到 example 分支
- [x] 6.2 创建或更新 README.md 说明这是完整示例
- [x] 6.3 添加快速启动指南（docker-compose up）
- [x] 6.4 说明如何切换到 main 分支查看框架代码
- [x] 6.5 添加开发工作流程说明

## 7. 更新 DEVELOPMENT.md

- [x] 7.1 添加双分支策略章节
- [x] 7.2 添加 example → main 反向工作流程说明
- [x] 7.3 添加提交消息规范章节（type 和 scope 列表）
- [x] 7.4 添加 cherry-pick 同步步骤和示例
- [x] 7.5 添加 git alias 配置说明
- [x] 7.6 添加常见问题和最佳实践

## 8. 更新 CLAUDE.md

- [x] 8.1 添加双分支策略说明
- [x] 8.2 说明 AI 应该在 example 分支开发
- [x] 8.3 添加提交消息格式要求
- [x] 8.4 说明如何同步框架提交到 main
- [x] 8.5 添加 git hooks 相关说明

## 9. 配置 Git Alias

- [x] 9.1 在 README 或 DEVELOPMENT.md 中提供 git alias 配置命令
- [x] 9.2 说明 fwlog、fwsync、ex、fm 等 alias 的用途
- [x] 9.3 提供全局和项目级配置的示例

## 10. 验证和测试

- [x] 10.1 在 main 分支验证目录结构正确
- [x] 10.2 在 example 分支验证完整项目可运行
- [x] 10.3 测试 git hooks 安装和验证功能
- [x] 10.4 测试提交消息格式验证（正确和错误情况）
- [x] 10.5 测试在 main 分支提交服务代码被拒绝
- [x] 10.6 测试 cherry-pick 工作流程
- [x] 10.7 验证 templates/ 目录内容完整

## 11. 提交变更

- [ ] 11.1 在 main 分支提交 templates/ 和文档更新
- [ ] 11.2 在 main 分支提交 scripts/git-hooks/ 和安装脚本
- [ ] 11.3 在 main 分支提交清理后的目录结构
- [ ] 11.4 在 example 分支提交 README 更新
- [ ] 11.5 验证两个分支的提交历史正确

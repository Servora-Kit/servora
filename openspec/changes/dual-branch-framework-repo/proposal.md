## 为什么

当前 servora 仓库混合了框架代码和示例服务，导致职责不清晰。在早期开发阶段，我们需要在完整环境中调试框架结构，但又要保持框架代码的纯净性以便作为 Go module 发布。采用双分支策略可以在同一仓库中同时满足这两个需求：main 分支保持纯净的框架代码用于发布，example 分支保留完整示例用于开发和测试。

## 变更内容

- **创建 example 分支**：保存当前完整项目结构（包含 servora 和 sayhello 两个示例服务及所有部署配置）
- **清理 main 分支**：只保留框架代码（pkg/、cmd/svr/、api/protos/、templates/、app.mk、docs/），删除服务实现和部署配置
- **创建 templates/ 目录**：从 manifests/ 提取配置模板（K8s、Docker、可观测性）
- **设置 git hooks**：强制提交消息规范（type(scope): description 格式），防止在 main 分支提交服务代码
- **创建辅助脚本**：支持 example → main 的反向工作流程（查看框架提交、同步到 main）
- **更新文档**：说明双分支策略、工作流程、提交规范

## 功能 (Capabilities)

### 新增功能
- `dual-branch-strategy`: 双分支仓库架构，main 分支为纯框架代码，example 分支为完整示例
- `commit-conventions`: 提交消息规范和 git hooks 强制执行
- `reverse-workflow`: example → main 反向工作流程和辅助工具

### 修改功能
<!-- 无现有功能需求变更 -->

## 影响

**代码结构**：
- main 分支：删除 app/、manifests/、docker-compose.yaml、go.work
- main 分支：新增 templates/ 目录
- example 分支：保持当前完整结构

**开发流程**：
- 主要开发在 example 分支进行
- 框架相关提交通过 cherry-pick 同步到 main
- 需要遵循严格的提交消息格式

**发布流程**：
- Go module 用户只看到 main 分支的纯框架代码
- example 分支不参与发布

**协作规范**：
- 所有提交必须遵循 type(scope): description 格式
- git hooks 自动验证提交消息和防止错误操作

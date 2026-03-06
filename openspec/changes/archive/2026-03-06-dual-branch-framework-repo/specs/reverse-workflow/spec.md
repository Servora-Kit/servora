## 新增需求

### 需求:主要开发必须在 example 分支进行

开发者必须在 example 分支进行主要开发工作，可以同时修改框架代码和服务代码，在完整环境中测试。

#### 场景:在 example 分支开发新功能
- **当** 开发者需要添加新的框架功能
- **那么** 系统必须允许在 example 分支同时修改 pkg/ 和 app/ 目录

#### 场景:在 example 分支测试框架修改
- **当** 开发者修改框架代码后
- **那么** 系统必须允许立即在 app/servora/ 或 app/sayhello/ 中测试该修改

#### 场景:在 example 分支运行完整环境
- **当** 开发者在 example 分支执行 make gen && docker-compose up
- **那么** 系统必须启动完整的开发环境用于测试

### 需求:框架提交必须通过 cherry-pick 同步到 main

当 example 分支包含框架相关的提交时，必须通过 git cherry-pick 将这些提交同步到 main 分支。

#### 场景:识别框架提交
- **当** 开发者在 example 分支查看提交历史
- **那么** 系统必须能够通过 scope (pkg, cmd) 识别哪些提交需要同步到 main

#### 场景:同步单个框架提交
- **当** 开发者在 main 分支执行 git cherry-pick <commit-hash>
- **那么** 系统必须将该框架提交应用到 main 分支

#### 场景:同步多个框架提交
- **当** 开发者在 main 分支执行 git cherry-pick <hash1> <hash2> <hash3>
- **那么** 系统必须按顺序将所有框架提交应用到 main 分支

### 需求:必须提供辅助工具查看框架提交

系统必须提供 git alias 或脚本，方便开发者快速查看需要同步的框架提交。

#### 场景:查看所有框架提交
- **当** 开发者在 example 分支执行 git fwlog
- **那么** 系统必须显示所有 scope 为 pkg 或 cmd 的提交

#### 场景:查看需要同步的框架提交
- **当** 开发者在 example 分支执行 git fwsync
- **那么** 系统必须显示 example 分支有但 main 分支没有的框架提交

#### 场景:快速切换分支
- **当** 开发者执行 git ex 或 git fm
- **那么** 系统必须快速切换到 example 或 main 分支

### 需求:提交必须小而专注

为了方便 cherry-pick，每个提交必须小而专注，只修改一个功能或修复一个问题。

#### 场景:单一职责的提交
- **当** 开发者修改 pkg/logger/logger.go 添加新功能
- **那么** 系统必须要求该提交只包含 logger 相关的修改，不包含其他文件

#### 场景:避免混合提交
- **当** 开发者同时修改 pkg/logger/logger.go 和 app/servora/service/internal/service/user.go
- **那么** 系统必须建议拆分为两个提交：一个 feat(pkg)，一个 feat(servora)

### 需求:必须提供工作流程文档

系统必须在 DEVELOPMENT.md 中详细说明 example → main 的反向工作流程。

#### 场景:查看工作流程说明
- **当** 开发者打开 DEVELOPMENT.md
- **那么** 系统必须包含完整的工作流程说明：在 example 开发、提交规范、同步到 main

#### 场景:查看提交规范
- **当** 开发者查看文档中的提交规范章节
- **那么** 系统必须列出所有有效的 type 和 scope，以及示例

#### 场景:查看同步步骤
- **当** 开发者查看文档中的同步章节
- **那么** 系统必须提供详细的 cherry-pick 步骤和示例命令

### 需求:CLAUDE.md 必须说明 AI 工作流程

系统必须在 CLAUDE.md 中说明 AI 协作时的工作流程和提交规范，确保 AI 遵循相同的规范。

#### 场景:AI 查看工作流程
- **当** AI 读取 CLAUDE.md
- **那么** 系统必须说明 AI 应该在 example 分支开发，并遵循提交消息格式

#### 场景:AI 提交代码
- **当** AI 尝试提交代码
- **那么** 系统必须通过 git hooks 强制 AI 遵循提交消息格式

#### 场景:AI 同步框架提交
- **当** AI 需要同步框架提交到 main
- **那么** 系统必须在 CLAUDE.md 中说明使用 cherry-pick 的步骤

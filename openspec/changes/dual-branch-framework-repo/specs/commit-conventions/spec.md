## 新增需求

### 需求:提交消息必须遵循规范格式

所有 git 提交消息必须遵循 `type(scope): description` 格式，其中 type 和 scope 必须是预定义的值。

#### 场景:提交框架代码
- **当** 用户修改 pkg/logger/logger.go 并提交
- **那么** 系统必须要求提交消息格式为 `feat(pkg): add structured logging` 或类似格式

#### 场景:提交服务代码
- **当** 用户修改 app/servora/service/internal/service/user.go 并提交
- **那么** 系统必须要求提交消息格式为 `feat(servora): add user service` 或类似格式

#### 场景:拒绝不规范的提交消息
- **当** 用户尝试提交消息 "add logger"（缺少 type 和 scope）
- **那么** 系统必须拒绝提交并显示错误消息和正确格式示例

### 需求:type 必须是预定义的值

提交消息的 type 字段必须是以下值之一：feat、fix、refactor、docs、test、chore。

#### 场景:使用有效的 type
- **当** 用户使用 type 为 feat、fix、refactor、docs、test 或 chore
- **那么** 系统必须接受该提交

#### 场景:拒绝无效的 type
- **当** 用户使用 type 为 "update" 或 "change"
- **那么** 系统必须拒绝提交并列出有效的 type 值

### 需求:scope 必须是预定义的值

提交消息的 scope 字段必须是以下值之一：pkg、cmd、servora、sayhello、example。

#### 场景:使用框架相关的 scope
- **当** 用户修改框架代码并使用 scope 为 pkg 或 cmd
- **那么** 系统必须接受该提交并标记为需要同步到 main 分支

#### 场景:使用服务相关的 scope
- **当** 用户修改服务代码并使用 scope 为 servora 或 sayhello
- **那么** 系统必须接受该提交并标记为只在 example 分支

#### 场景:拒绝无效的 scope
- **当** 用户使用 scope 为 "core" 或 "utils"
- **那么** 系统必须拒绝提交并列出有效的 scope 值

### 需求:git hooks 必须自动验证提交消息

系统必须通过 commit-msg hook 自动验证提交消息格式，在提交时立即检查，而不是在 CI 阶段。

#### 场景:安装 git hooks
- **当** 用户执行 ./scripts/install-hooks.sh
- **那么** 系统必须将 commit-msg hook 复制到 .git/hooks/ 并设置可执行权限

#### 场景:hooks 自动验证提交
- **当** 用户执行 git commit
- **那么** 系统必须在提交前自动运行 commit-msg hook 验证消息格式

#### 场景:hooks 提供清晰的错误消息
- **当** 提交消息格式错误
- **那么** 系统必须显示错误原因、正确格式说明和示例

### 需求:禁止在 main 分支提交服务代码

系统必须通过 pre-commit hook 防止在 main 分支直接提交服务相关代码（app/ 目录）。

#### 场景:在 main 分支修改框架代码
- **当** 用户在 main 分支修改 pkg/logger/logger.go 并提交
- **那么** 系统必须允许提交

#### 场景:在 main 分支修改服务代码
- **当** 用户在 main 分支修改 app/servora/service/internal/service/user.go 并提交
- **那么** 系统必须拒绝提交并提示应该在 example 分支修改服务代码

#### 场景:在 example 分支修改任何代码
- **当** 用户在 example 分支修改任何文件并提交
- **那么** 系统必须允许提交（不限制）

### 需求:hooks 脚本必须可跟踪和安装

git hooks 脚本必须存放在 scripts/git-hooks/ 目录中，可以被 git 跟踪，并提供安装脚本。

#### 场景:查看 hooks 脚本
- **当** 用户访问 scripts/git-hooks/ 目录
- **那么** 系统必须包含 commit-msg 和 pre-commit 脚本文件

#### 场景:安装 hooks
- **当** 用户执行 ./scripts/install-hooks.sh
- **那么** 系统必须复制所有 hooks 到 .git/hooks/ 并设置可执行权限

#### 场景:README 说明 hooks 安装
- **当** 用户查看 README.md 或 DEVELOPMENT.md
- **那么** 系统必须包含 hooks 安装步骤说明

## 新增需求

### 需求:通用化 commit scope 规则

commit-msg hook 必须使用通用的 scope 类别（pkg、cmd、app、example、openspec、infra）而不是具体的服务名称。

#### 场景:接受通用 app scope

- **当** 开发者提交消息为 "feat(app): add new feature"
- **那么** commit-msg hook 必须验证通过

#### 场景:拒绝旧的具体服务名

- **当** 开发者提交消息为 "feat(servora): add new feature"
- **那么** commit-msg hook 必须拒绝提交并提示使用 app scope

### 需求:通用化 pre-commit 路径检查

pre-commit hook 必须检查 app/ 目录下的所有文件，而不是硬编码具体的服务名称。

#### 场景:main 分支禁止提交 app 目录

- **当** 开发者在 main 分支尝试提交 app/anyservice/ 下的文件
- **那么** pre-commit hook 必须拒绝提交并提示切换到 example 分支

#### 场景:example 分支允许提交 app 目录

- **当** 开发者在 example 分支提交 app/anyservice/ 下的文件
- **那么** pre-commit hook 必须允许提交

### 需求:pre-commit 执行 gofmt 检查

pre-commit hook 必须对所有 .go 文件执行 gofmt 格式检查，确保代码格式一致。

#### 场景:格式正确的代码通过检查

- **当** 开发者提交格式正确的 .go 文件
- **那么** pre-commit hook 必须验证通过

#### 场景:格式错误的代码被拒绝

- **当** 开发者提交格式不正确的 .go 文件
- **那么** pre-commit hook 必须拒绝提交并显示需要格式化的文件列表

### 需求:pre-commit 保持快速执行

pre-commit hook 的执行时间必须控制在 1 秒左右，不影响开发体验。

#### 场景:快速分支检查

- **当** pre-commit hook 执行分支检查
- **那么** 检查必须在 100ms 内完成

#### 场景:快速 gofmt 检查

- **当** pre-commit hook 对少量文件执行 gofmt 检查
- **那么** 检查必须在 1 秒内完成

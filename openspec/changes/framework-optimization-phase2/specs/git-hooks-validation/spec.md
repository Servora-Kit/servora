## Purpose
定义 git-hooks-validation 的功能需求和验证场景。

## Requirements

### Requirement: 通用化 commit scope 规则

commit-msg hook 必须使用通用的 scope 类别（pkg、cmd、app、example、openspec、infra）而不是具体的服务名称。

#### Scenario: 接受通用 app scope

- **WHEN** 开发者提交消息为 "feat(app): add new feature"
- **THEN** commit-msg hook 必须验证通过

#### Scenario: 拒绝旧的具体服务名

- **WHEN** 开发者提交消息为 "feat(servora): add new feature"
- **THEN** commit-msg hook 必须拒绝提交并提示使用 app scope

### Requirement: 通用化 pre-commit 路径检查

pre-commit hook 必须检查 app/ 目录下的所有文件，而不是硬编码具体的服务名称。

#### Scenario: main 分支禁止提交 app 目录

- **WHEN** 开发者在 main 分支尝试提交 app/anyservice/ 下的文件
- **THEN** pre-commit hook 必须拒绝提交并提示切换到 example 分支

#### Scenario: example 分支允许提交 app 目录

- **WHEN** 开发者在 example 分支提交 app/anyservice/ 下的文件
- **THEN** pre-commit hook 必须允许提交

### Requirement: pre-commit 执行 gofmt 检查

pre-commit hook 必须对所有 .go 文件执行 gofmt 格式检查，确保代码格式一致。

#### Scenario: 格式正确的代码通过检查

- **WHEN** 开发者提交格式正确的 .go 文件
- **THEN** pre-commit hook 必须验证通过

#### Scenario: 格式错误的代码被拒绝

- **WHEN** 开发者提交格式不正确的 .go 文件
- **THEN** pre-commit hook 必须拒绝提交并显示需要格式化的文件列表

### Requirement: pre-commit 保持快速执行

pre-commit hook 的执行时间必须控制在 1 秒左右，不影响开发体验。

#### Scenario: 快速分支检查

- **WHEN** pre-commit hook 执行分支检查
- **THEN** 检查必须在 100ms 内完成

#### Scenario: 快速 gofmt 检查

- **WHEN** pre-commit hook 对少量文件执行 gofmt 检查
- **THEN** 检查必须在 1 秒内完成

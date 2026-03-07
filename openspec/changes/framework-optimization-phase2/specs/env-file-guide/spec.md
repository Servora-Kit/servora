## Purpose
定义 env-file-guide 的功能需求和验证场景。

## Requirements

### Requirement: .env 文件包含详细使用说明

example 分支的 .env 文件必须在文件顶部包含详细的注释，说明敏感信息管理、.env.local 的使用和安全最佳实践。

#### Scenario: 敏感信息警告

- **WHEN** 开发者打开 .env 文件
- **THEN** 文件必须在顶部包含明确的警告注释，禁止提交真实的敏感信息（密码、密钥、token）

#### Scenario: .env.local 使用说明

- **WHEN** 开发者查看 .env 文件的注释
- **THEN** 注释必须说明应该创建 .env.local 文件存储本地敏感配置，并且 .env.local 已被 .gitignore 忽略

### Requirement: .env 文件使用示例值

example 分支的 .env 文件必须只包含示例值或占位符，不包含真实的敏感信息。

#### Scenario: 数据库密码使用占位符

- **WHEN** 开发者查看 .env 文件中的数据库配置
- **THEN** 密码字段必须使用占位符（如 "your_password_here"）而不是真实密码

#### Scenario: API 密钥使用示例值

- **WHEN** 开发者查看 .env 文件中的 API 配置
- **THEN** API 密钥字段必须使用示例值（如 "sk_test_example"）而不是真实密钥

### Requirement: .gitignore 包含 .env.local

项目的 .gitignore 文件必须包含 .env.local 条目，防止本地敏感配置被意外提交。

#### Scenario: .env.local 被忽略

- **WHEN** 开发者创建 .env.local 文件并执行 git status
- **THEN** .env.local 必须不出现在未跟踪文件列表中

#### Scenario: .env 可以被提交

- **WHEN** 开发者修改 .env 文件（仅包含示例值）并执行 git status
- **THEN** .env 必须可以被正常跟踪和提交

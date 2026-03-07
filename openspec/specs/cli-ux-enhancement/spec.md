## Purpose
定义 cli-ux-enhancement 的功能需求和验证场景。

## Requirements

### Requirement: 美化输出样式

系统必须使用 `charmbracelet/lipgloss` 美化 CLI 输出，提供清晰易读的视觉体验。

#### Scenario: 成功消息输出

- **WHEN** 生成成功完成
- **THEN** 系统使用绿色样式输出成功标记（✓）
- **THEN** 标题使用粗体样式
- **THEN** 路径信息使用高亮样式

#### Scenario: 错误消息输出

- **WHEN** 生成失败
- **THEN** 系统使用红色样式输出错误标记（✗）
- **THEN** 错误信息清晰可读
- **THEN** 提供的建议使用不同的样式区分

#### Scenario: 进度信息输出

- **WHEN** 处理多个服务时
- **THEN** 系统以微服务为最小单位输出当前处理的服务名称
- **THEN** 使用进度指示器（如 "[1/3] generating servora"）
- **THEN** 最终输出总结信息（成功/失败统计）
- **THEN** 失败明细逐条展示（服务名 + 错误信息）

### Requirement: 输出格式一致性

系统必须保持输出格式的一致性，参考 Ech0 项目的设计模式。

#### Scenario: 标题和消息格式

- **WHEN** 输出任何信息
- **THEN** 使用 "标题: 消息" 的格式
- **THEN** 标题使用粗体和特定颜色
- **THEN** 消息使用高亮样式

#### Scenario: 多行信息输出

- **WHEN** 需要输出多行信息（如错误建议）
- **THEN** 使用适当的缩进
- **THEN** 保持视觉层次清晰

### Requirement: 交互式服务选择

系统必须使用 `charmbracelet/huh` 提供无参数时的交互式服务选择体验。

#### Scenario: 无参数触发交互

- **WHEN** 用户执行 `svr gen gorm` 不带参数
- **THEN** 系统显示交互式服务选择界面
- **THEN** 用户可以通过方向键选择服务
- **THEN** 支持多选模式

#### Scenario: 确认提示

- **WHEN** 即将执行生成操作
- **THEN** 系统显示确认提示
- **THEN** 显示将要生成的服务列表
- **THEN** 用户可以确认或取消操作

#### Scenario: 空选择退出

- **WHEN** 用户在交互式服务选择中未选择任何服务
- **THEN** 系统输出 "No services selected"
- **THEN** 系统不执行任何生成操作
- **THEN** 系统以退出码 0 结束

#### Scenario: 用户取消

- **WHEN** 用户在确认阶段取消操作
- **THEN** 系统输出 "Cancelled by user"
- **THEN** 系统不执行任何生成操作
- **THEN** 系统以退出码 0 结束

### Requirement: 颜色适配

系统必须支持浅色和深色终端主题。

#### Scenario: 自适应颜色

- **WHEN** 在不同终端主题下运行
- **THEN** 系统使用 `lipgloss.AdaptiveColor` 自动适配
- **THEN** 浅色主题使用深色文字
- **THEN** 深色主题使用浅色文字
- **THEN** 保持可读性

### Requirement: 汇总输出与错误分类

系统必须在批量执行结束后输出可读且可解析的汇总结果，并对失败进行标准化分类。

#### Scenario: 汇总输出格式

- **WHEN** 系统完成批量生成
- **THEN** 第一行输出 `Summary: success=X failed=Y`
- **THEN** 每条失败项使用 `- <service> [<error-type>] <message>` 格式输出

#### Scenario: 失败分类

- **WHEN** 某服务执行失败
- **THEN** 系统将错误分类为以下之一：`service-not-found`、`config-invalid`、`db-connect-failed`、`generation-failed`
- **THEN** 汇总失败项中必须包含对应 `error-type`

#### Scenario: 退出码语义

- **WHEN** 批量执行无任何失败
- **THEN** 系统以退出码 0 结束
- **WHEN** 批量执行存在任意失败
- **THEN** 系统以退出码 1 结束

### Requirement: CLI 必须暴露受支持的生成命令

`svr` CLI 必须将当前受支持的生成能力暴露为清晰的子命令集合，避免文档与帮助信息出现不一致。

#### Scenario: 根命令帮助展示生成命令
- **WHEN** 用户执行 `svr --help`
- **THEN** 帮助信息必须至少展示 `gen` 命令组
- **THEN** 帮助信息必须展示 `new` 命令组

#### Scenario: new 命令组展示 api 子命令
- **WHEN** 用户执行 `svr new --help`
- **THEN** 帮助信息必须展示 `api` 子命令
- **THEN** `api` 子命令说明必须体现其服务维度输入方式

## 移除需求

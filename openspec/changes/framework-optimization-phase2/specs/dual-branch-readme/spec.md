## Purpose
定义 dual-branch-readme 的功能需求和验证场景。

## Requirements

### Requirement: main 分支 README 展示框架能力

main 分支的 README.md 必须聚焦于框架本身的能力、架构设计和扩展方式，不包含具体业务服务的运行说明。

#### Scenario: 框架概述

- **WHEN** 开发者在 main 分支查看 README.md
- **THEN** 文档必须包含框架的核心特性、技术栈、架构设计和目录结构说明

#### Scenario: 快速开始指向 example 分支

- **WHEN** 开发者在 main 分支查看 README.md 的快速开始章节
- **THEN** 文档必须引导开发者切换到 example 分支查看完整的运行示例

### Requirement: example 分支 README 展示完整项目

example 分支的 README.md 必须包含完整项目的运行指南、开发流程和示例服务说明。

#### Scenario: 完整运行指南

- **WHEN** 开发者在 example 分支查看 README.md
- **THEN** 文档必须包含环境准备、依赖安装、服务启动、API 测试的完整步骤

#### Scenario: 示例服务说明

- **WHEN** 开发者在 example 分支查看 README.md
- **THEN** 文档必须说明 servora 和 sayhello 两个示例服务的功能和使用方式

### Requirement: 分支差异化内容管理

README.md 必须在两个分支中维护不同的内容，main 分支的更新不应覆盖 example 分支的运行指南。

#### Scenario: main 分支更新不影响 example

- **WHEN** 开发者在 main 分支更新框架说明
- **THEN** example 分支的 README.md 必须保持其完整项目运行指南不变

#### Scenario: example 分支更新不影响 main

- **WHEN** 开发者在 example 分支更新运行指南
- **THEN** main 分支的 README.md 必须保持其框架能力说明不变

## Purpose
定义 git-hooks-sync 的功能需求和验证场景。

## Requirements

### Requirement: 自动同步 Git Hooks

系统必须在分支切换或合并后自动同步 scripts/git-hooks/ 目录中的 hooks 到 .git/hooks/ 目录，确保开发者始终使用最新版本的 hooks。

#### Scenario: 分支切换后自动同步

- **WHEN** 开发者从 main 分支切换到 example 分支
- **THEN** 系统必须自动执行 scripts/install-hooks.sh，将最新的 hooks 复制到 .git/hooks/

#### Scenario: 合并后自动同步

- **WHEN** 开发者合并了包含 hooks 更新的提交
- **THEN** 系统必须自动执行 scripts/install-hooks.sh，更新 .git/hooks/ 中的 hooks

### Requirement: 符号链接支持

install-hooks.sh 脚本必须支持创建符号链接，使 .git/hooks/ 中的 hooks 直接指向 scripts/git-hooks/ 中的源文件。

#### Scenario: 使用符号链接安装

- **WHEN** 开发者运行 scripts/install-hooks.sh --symlink
- **THEN** 系统必须在 .git/hooks/ 中创建指向 scripts/git-hooks/ 的符号链接，而不是复制文件

#### Scenario: 符号链接自动更新

- **WHEN** scripts/git-hooks/ 中的 hook 文件被修改
- **THEN** .git/hooks/ 中的符号链接必须自动反映最新内容，无需重新安装

### Requirement: post-merge Hook 触发

系统必须提供 post-merge hook，在 git merge 或 git pull 后自动触发 hooks 同步。

#### Scenario: merge 后触发同步

- **WHEN** 开发者执行 git merge 并且 scripts/git-hooks/ 目录有变更
- **THEN** post-merge hook 必须自动执行 scripts/install-hooks.sh

#### Scenario: pull 后触发同步

- **WHEN** 开发者执行 git pull 并且 scripts/git-hooks/ 目录有变更
- **THEN** post-merge hook 必须自动执行 scripts/install-hooks.sh

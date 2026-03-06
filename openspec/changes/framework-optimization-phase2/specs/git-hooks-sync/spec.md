## 新增需求

### 需求:自动同步 Git Hooks

系统必须在分支切换或合并后自动同步 scripts/git-hooks/ 目录中的 hooks 到 .git/hooks/ 目录，确保开发者始终使用最新版本的 hooks。

#### 场景:分支切换后自动同步

- **当** 开发者从 main 分支切换到 example 分支
- **那么** 系统必须自动执行 scripts/install-hooks.sh，将最新的 hooks 复制到 .git/hooks/

#### 场景:合并后自动同步

- **当** 开发者合并了包含 hooks 更新的提交
- **那么** 系统必须自动执行 scripts/install-hooks.sh，更新 .git/hooks/ 中的 hooks

### 需求:符号链接支持

install-hooks.sh 脚本必须支持创建符号链接，使 .git/hooks/ 中的 hooks 直接指向 scripts/git-hooks/ 中的源文件。

#### 场景:使用符号链接安装

- **当** 开发者运行 scripts/install-hooks.sh --symlink
- **那么** 系统必须在 .git/hooks/ 中创建指向 scripts/git-hooks/ 的符号链接，而不是复制文件

#### 场景:符号链接自动更新

- **当** scripts/git-hooks/ 中的 hook 文件被修改
- **那么** .git/hooks/ 中的符号链接必须自动反映最新内容，无需重新安装

### 需求:post-merge Hook 触发

系统必须提供 post-merge hook，在 git merge 或 git pull 后自动触发 hooks 同步。

#### 场景:merge 后触发同步

- **当** 开发者执行 git merge 并且 scripts/git-hooks/ 目录有变更
- **那么** post-merge hook 必须自动执行 scripts/install-hooks.sh

#### 场景:pull 后触发同步

- **当** 开发者执行 git pull 并且 scripts/git-hooks/ 目录有变更
- **那么** post-merge hook 必须自动执行 scripts/install-hooks.sh

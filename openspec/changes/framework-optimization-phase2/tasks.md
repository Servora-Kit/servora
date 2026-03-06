## 1. Git Hooks 通用化

- [x] 1.1 更新 scripts/git-hooks/commit-msg，将 scope 正则从具体服务名改为通用类别（pkg|cmd|app|example|openspec|infra）
- [x] 1.2 更新 scripts/git-hooks/pre-commit，将路径检查从 `app/(servora|sayhello)/` 改为 `app/`
- [x] 1.3 在 scripts/git-hooks/pre-commit 中添加 gofmt 格式检查，只检查 staged 的 .go 文件
- [x] 1.4 测试 commit-msg hook：尝试提交 feat(app): test 和 feat(servora): test，验证前者通过后者拒绝
- [x] 1.5 测试 pre-commit hook：在 main 分支尝试提交 app/ 文件，验证被拒绝

## 2. Git Hooks 同步机制

- [x] 2.1 更新 scripts/install-hooks.sh，添加 --symlink 参数支持，默认使用符号链接
- [x] 2.2 在 install-hooks.sh 中添加 Windows 兼容性检测，符号链接失败时回退到文件复制
- [x] 2.3 创建 scripts/git-hooks/post-merge hook，检测 scripts/git-hooks/ 变更后自动执行 install-hooks.sh
- [x] 2.4 更新 .git/hooks/ 中的所有 hooks，使用符号链接指向 scripts/git-hooks/
- [x] 2.5 测试 post-merge hook：修改 scripts/git-hooks/commit-msg，执行 git pull，验证 .git/hooks/commit-msg 自动更新

## 3. README 双分支差异化

- [x] 3.1 在 main 分支更新 README.md，聚焦框架能力、架构设计、技术栈说明
- [x] 3.2 在 main 分支 README.md 的快速开始章节，引导开发者切换到 example 分支
- [x] 3.3 切换到 example 分支，更新 README.md，包含完整的运行指南、环境准备、服务启动步骤
- [x] 3.4 在 example 分支 README.md 中添加 servora 和 sayhello 示例服务的功能说明
- [x] 3.5 在 CLAUDE.md 中添加 README 合并策略说明：保留目标分支的 README 内容

## 4. .env 文件文档化

- [x] 4.1 切换到 example 分支，在 .env 文件顶部添加详细的多行注释
- [x] 4.2 在 .env 注释中说明禁止提交真实敏感信息，应使用 .env.local 存储本地配置
- [x] 4.3 确保 .env 文件中的所有敏感字段使用占位符（如 "your_password_here"）
- [x] 4.4 在 .gitignore 中添加 .env.local 条目
- [x] 4.5 测试 .env.local：创建该文件，执行 git status，验证不出现在未跟踪文件列表中

## 5. 文档更新

- [x] 5.1 在 CLAUDE.md 中更新 Git Hooks scope 说明，反映新的通用类别
- [x] 5.2 在 CLAUDE.md 中添加 Git Hooks 同步机制说明（符号链接 + post-merge）
- [x] 5.3 在 CLAUDE.md 中添加双分支 README 策略说明
- [x] 5.4 创建 CI/CD 设计文档（docs/cicd-design.md），说明 main 和 example 分支的不同 workflow

## 6. 验证和测试

- [ ] 6.1 在 main 分支执行 make lint.go，验证代码格式正确
- [ ] 6.2 在 example 分支执行 make lint.go && make test，验证所有测试通过
- [ ] 6.3 测试完整的分支切换流程：main → example → main，验证 hooks 自动同步
- [ ] 6.4 测试提交流程：在两个分支分别提交代码，验证 scope 规则和路径检查正确
- [ ] 6.5 验证 README 内容：确认 main 分支展示框架能力，example 分支展示完整项目

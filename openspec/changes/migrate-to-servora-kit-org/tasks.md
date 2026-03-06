## 1. 准备与备份

- [ ] 1.1 创建 git commit 备份当前状态
- [ ] 1.2 验证当前构建正常（运行 `make gen && make test`）
- [ ] 1.3 确认所有需要更新的文件列表（使用 grep 搜索 `horonlee/servora`）

## 2. 更新 Proto 文件

- [ ] 2.1 更新所有共享 proto 文件（`api/protos/`）的 `go_package` 选项，从 `github.com/horonlee/servora` 改为 `github.com/Servora-Kit/servora`
- [ ] 2.2 更新 servora 服务 proto 文件（`app/servora/service/api/protos/`）的 `go_package` 选项
- [ ] 2.3 更新 sayhello 服务 proto 文件（`app/sayhello/service/api/protos/`）的 `go_package` 选项
- [ ] 2.4 更新 proto 文件中的 OpenAPI contact URL（`template_doc.proto`、`servora_doc.proto`、`sayhello_doc.proto`）

## 3. 更新 Go 模块文件

- [ ] 3.1 更新根目录 `go.mod` 的 module 路径为 `github.com/Servora-Kit/servora`
- [ ] 3.2 更新 `api/gen/go.mod` 的 module 路径为 `github.com/Servora-Kit/servora/api/gen`
- [ ] 3.3 更新 `app/servora/service/go.mod` 的 module 路径和 require 语句
- [ ] 3.4 更新 `app/sayhello/service/go.mod` 的 module 路径和 require 语句
- [ ] 3.5 更新 `go.work` 文件（如果包含路径引用）

## 4. 更新 Go 源代码

- [ ] 4.1 更新 `pkg/` 目录下所有 Go 文件的 import 语句
- [ ] 4.2 更新 `cmd/svr/` 目录下所有 Go 文件的 import 语句
- [ ] 4.3 更新 `app/servora/service/` 目录下所有 Go 文件的 import 语句
- [ ] 4.4 更新 `app/sayhello/service/` 目录下所有 Go 文件的 import 语句

## 5. 更新配置文件

- [ ] 5.1 更新 `app/servora/service/api/buf.openapi.gen.yaml` 中的路径引用
- [ ] 5.2 更新 `app/sayhello/service/api/buf.openapi.gen.yaml` 中的路径引用
- [ ] 5.3 检查并更新其他 YAML 配置文件中的路径引用

## 6. 更新文档

- [ ] 6.1 更新 `README.md` 中的 GitHub 仓库链接
- [ ] 6.2 更新根目录 `AGENTS.md` 中的链接和路径引用
- [ ] 6.3 更新 `cmd/svr/AGENTS.md` 中的路径引用
- [ ] 6.4 更新 `api/AGENTS.md` 和 `api/protos/AGENTS.md` 中的路径引用
- [ ] 6.5 更新 `CLAUDE.md` 中的路径引用（如果有）

## 7. 更新 OpenSpec 规范

- [ ] 7.1 更新 `openspec/specs/` 目录下现有规范文件中的路径引用
- [ ] 7.2 保持 `openspec/changes/archive/` 中的历史文件不变（作为历史记录）

## 8. 重新生成代码

- [ ] 8.1 清理生成目录：`rm -rf api/gen/go`
- [ ] 8.2 运行 `make api` 重新生成 proto Go 代码
- [ ] 8.3 运行 `make wire` 重新生成所有服务的依赖注入代码
- [ ] 8.4 运行 `make ent` 重新生成 Ent 代码（如果需要）

## 9. 验证

- [ ] 9.1 使用 grep 确认所有 `horonlee/servora` 引用已更新（排除归档文件）
- [ ] 9.2 运行 `make test` 确保所有测试通过
- [ ] 9.3 运行 `make lint.go` 确保代码规范检查通过
- [ ] 9.4 运行 `make build` 确保所有服务能够正常构建
- [ ] 9.5 手动检查关键文件的路径是否正确（go.mod、proto 文件、主要源文件）

## 10. 提交变更

- [ ] 10.1 创建 git commit 记录所有变更
- [ ] 10.2 验证 commit 包含所有必要的文件
- [ ] 10.3 推送到远程仓库（如果需要）

## 新增需求

### 需求:支持 GitHub 组织账户迁移

系统必须支持从个人 GitHub 账户迁移到组织账户，包括自动更新所有代码引用、配置文件和文档中的仓库路径。

#### 场景:更新 Go 模块路径
- **当** 项目从个人账户迁移到组织账户
- **那么** 系统必须将所有 go.mod 文件中的 module 路径从 `github.com/horonlee/servora` 更新为 `github.com/Servora-Kit/servora`
- **那么** 系统必须将所有 go.mod 文件中的 replace 指令路径相应更新

#### 场景:更新 Go 源代码 import
- **当** 项目从个人账户迁移到组织账户
- **那么** 系统必须将所有 Go 源代码文件中的 import 语句从 `github.com/horonlee/servora` 更新为 `github.com/Servora-Kit/servora`
- **那么** 系统必须确保所有子包的 import 路径都正确更新

#### 场景:更新文档链接
- **当** 项目从个人账户迁移到组织账户
- **那么** 系统必须将 README.md 中的 GitHub 链接从 `github.com/horonlee/servora` 更新为 `github.com/Servora-Kit/servora`
- **那么** 系统必须将所有 AGENTS.md 文件中的 GitHub 链接相应更新

#### 场景:更新 OpenAPI 配置
- **当** 项目从个人账户迁移到组织账户
- **那么** 系统必须将 buf.openapi.gen.yaml 文件中的 contact URL 从旧路径更新为新路径

#### 场景:验证迁移完整性
- **当** 迁移完成后
- **那么** 系统必须确保没有遗留的旧路径引用（除归档文件外）
- **那么** 系统必须确保 `make gen` 和 `make test` 命令正常工作

## 修改需求

## 移除需求

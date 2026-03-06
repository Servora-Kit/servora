## 新增需求

## 修改需求

### 需求:必须创建 go.work 聚合所有模块

系统必须在项目根目录创建 `go.work` 文件，聚合所有 Go 模块（根模块、api/gen、servora、sayhello），支持本地多模块开发。

#### 场景:go.work 包含所有模块

- **当** 在根目录创建 `go.work`
- **那么** 必须使用 `use` 指令包含以下模块：
  - `.` (根模块)
  - `./api/gen` (生成代码模块)
  - `./app/servora/service` (servora 服务)
  - `./app/sayhello/service` (sayhello 服务)

#### 场景:go.work 提交到 Git

- **当** 创建 `go.work`
- **那么** 必须从 `.gitignore` 中移除 `go.work` 忽略规则
- **那么** `go.work` 必须提交到 Git，确保所有开发者和 CI 使用相同配置

#### 场景:本地依赖自动解析

- **当** 存在 `go.work` 并包含所有模块
- **那么** 服务模块引用 `github.com/Servora-Kit/servora/api/gen` 时必须自动解析到本地 `api/gen` 模块
- **那么** 服务模块引用 `github.com/Servora-Kit/servora` 时必须自动解析到本地根模块
- **那么** 不需要在服务 `go.mod` 中添加 `replace` 指令

## 移除需求

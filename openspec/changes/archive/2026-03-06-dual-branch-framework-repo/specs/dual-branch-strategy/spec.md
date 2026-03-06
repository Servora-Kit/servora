## 新增需求

### 需求:仓库必须包含 main 和 example 两个分支

仓库必须维护两个长期分支：main 分支只包含纯框架代码，example 分支包含完整的示例项目（框架代码 + 示例服务 + 部署配置）。

#### 场景:查看 main 分支内容
- **当** 用户切换到 main 分支
- **那么** 系统必须只显示框架相关目录：pkg/、cmd/svr/、api/protos/、templates/、app.mk、docs/、openspec/

#### 场景:查看 example 分支内容
- **当** 用户切换到 example 分支
- **那么** 系统必须显示完整项目结构：包含 app/servora/、app/sayhello/、manifests/、docker-compose.yaml、go.work

#### 场景:main 分支不包含服务代码
- **当** 用户在 main 分支查找服务目录
- **那么** 系统禁止存在 app/servora/ 或 app/sayhello/ 目录

### 需求:templates 目录必须包含配置模板

main 分支必须包含 templates/ 目录，提供 K8s、Docker 和可观测性的配置模板，供新项目参考使用。

#### 场景:查看 K8s 模板
- **当** 用户访问 templates/k8s/ 目录
- **那么** 系统必须提供 base/ 和 service/ 子目录，包含 namespace、rbac、deployment、service 等模板文件

#### 场景:查看 Docker 模板
- **当** 用户访问 templates/docker/ 目录
- **那么** 系统必须提供 Dockerfile 和 docker-compose.yaml 模板文件

#### 场景:查看可观测性模板
- **当** 用户访问 templates/observability/ 目录
- **那么** 系统必须提供 grafana/、loki/、otel/、prometheus/ 配置模板

### 需求:example 分支必须保持完整的工作示例

example 分支必须包含两个完整的可运行微服务示例（servora 和 sayhello），以及完整的本地开发和 K8s 部署配置。

#### 场景:运行本地开发环境
- **当** 用户在 example 分支执行 docker-compose up
- **那么** 系统必须启动所有服务（servora、sayhello、consul、数据库、redis、可观测性栈）

#### 场景:查看服务实现
- **当** 用户在 example 分支访问 app/servora/service/ 目录
- **那么** 系统必须包含完整的服务实现：internal/、cmd/、api/、configs/

#### 场景:查看部署配置
- **当** 用户在 example 分支访问 manifests/k8s/ 目录
- **那么** 系统必须包含 servora 和 sayhello 的完整 K8s 部署配置

### 需求:README 必须说明分支用途

两个分支的 README.md 必须清楚说明各自的用途和使用方式，避免用户混淆。

#### 场景:查看 main 分支 README
- **当** 用户在 main 分支打开 README.md
- **那么** 系统必须说明这是框架仓库，并引导用户查看 example 分支获取完整示例

#### 场景:查看 example 分支 README
- **当** 用户在 example 分支打开 README.md
- **那么** 系统必须说明这是完整示例，并说明如何快速启动和运行

### 需求:Go module 发布只使用 main 分支

当框架作为 Go module 被其他项目依赖时，必须只暴露 main 分支的代码，example 分支不参与发布。

#### 场景:通过 go get 安装框架
- **当** 用户执行 go get github.com/Servora-Kit/servora@latest
- **那么** 系统必须只下载 main 分支的框架代码，不包含示例服务

#### 场景:查看 go.mod 依赖
- **当** 用户在自己的项目中依赖 servora 框架
- **那么** 系统必须只引入 pkg/、cmd/svr/ 等框架包，不引入 app/ 目录

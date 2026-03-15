# AGENTS.md - manifests/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 概览
统一部署清单，K8s已收敛到 manifests/k8s/。

## 结构
```
manifests/
├── k8s/
│   ├── base/           # 基础设施（etcd/redis/postgres）
│   ├── servora/        # 主服务清单
│   └── sayhello/       # 示例服务清单
├── scripts/            # 脚本与自动化（k6 压测、postgres-init）
│   ├── k6/             # 压测脚本
│   └── postgres-init/  # Compose 用 DB 初始化 SQL
└── ...
```

## WHERE TO LOOK

| 任务 | 位置 | 说明 |
|------|------|------|
| 本地基础设施 | ../docker-compose.yaml | 根目录 make compose.up |
| 本地开发环境 | ../docker-compose.yaml + ../docker-compose.dev.yaml | 根目录 make compose.dev |
| K8s基础设施 | k8s/base/ | etcd/redis/postgres StatefulSet |
| 服务部署 | k8s/{service}/ | Deployment + Service + ConfigMap |
| 数据库初始化 | app/iam/service/manifests/（如有） | SQL 初始化脚本 |
| Postgres 初始化（Compose） | scripts/postgres-init/ | 根 docker-compose 挂载 |
| 压测脚本 | scripts/ | k6 |

## 约定

### 与templates/区别
- **templates/** - 框架级模板（main分支，给用户参考）
- **manifests/** - 可运行配置（example分支，基于templates创建）
- 两者不同步

### K8s清单组织
- base/ 包含 kustomization.yaml 聚合基础设施
- 服务目录包含 deployment.yaml + service.yaml + configmap.yaml
- 使用 kubectl apply -k 部署

### 开发流程
```bash
make compose.up         # 启动基础设施
make compose.dev        # 启动开发栈+日志
make compose.stop       # 仅停止基础设施容器
make compose.dev.stop   # 仅停止开发栈容器
make compose.dev.build  # 构建开发镜像
make compose.down       # 移除本地 Compose 栈（保留数据卷）
make compose.reset      # 移除本地 Compose 栈（含数据卷）
make compose.dev.down   # 移除开发栈（保留数据卷）
make compose.dev.reset  # 移除开发栈（含数据卷）
```

## 注意事项
- SQL 初始化脚本若在服务目录则位于该服务 `manifests/`，不在此目录
- K8s清单使用 kustomize 管理，不是 Helm
- 根 `docker-compose.yaml` 仅包含基础设施；Air 热重载开发容器位于根 `docker-compose.dev.yaml`

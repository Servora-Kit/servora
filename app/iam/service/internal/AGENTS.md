# AGENTS.md - app/iam/service/internal/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 目录定位

`internal/` 是 IAM 服务的核心实现层，按 `service -> biz -> data` 分层，`server/` 负责入口装配，`oidc/` 负责 OIDC 登录。

## 当前结构

```text
internal/
├── biz/          # 用例与仓储接口（authn, user, organization, project, application）
├── data/         # Ent / GORM 实现与 schema
├── oidc/         # OIDC 登录（login, callback）
├── server/       # HTTP/gRPC、中间件、注册、遥测
└── service/      # gRPC/HTTP 接口实现（authn, authz, user, org, project 等）
```

## 分层规则

- `service/`：协议适配与参数转换，错误仅做 `err != nil` 判断
- `biz/`：业务规则、用例编排、仓储接口；错误用 proto/Kratos errors 包装
- `data/`：Ent / Redis / 服务发现等具体实现
- `server/`：HTTP、gRPC、中间件、注册、指标装配
- `oidc/`：OIDC 授权码流程、登录页、回调

## 常用命令

在 `app/iam/service/` 目录执行：

```bash
make wire
make gen.ent
make gen.gorm
go test ./internal/...
```

## 维护提示

- 新增业务模块时通常需同时改 `biz/`、`data/`、`service/`，并视情况在 `server/` 注册
- 修改 ProviderSet 后必须执行 `make wire`

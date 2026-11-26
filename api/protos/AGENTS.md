# AGENTS.md - api/protos/

<!-- Parent: ../../AGENTS.md -->
<!-- Updated: 2026-05-24 -->

## 当前定位

`api/protos/` 是 Servora 框架公共 proto contract 根，随仓库根 `buf.yaml` 发布到 `buf.build/servora/servora`。

这里定义框架级 annotation、配置 schema 与通用数据结构。

## 当前结构

```text
api/protos/
├── README.md
├── AGENTS.md
└── servora/
    ├── conf/v1/                     # config annotation extensions
    ├── core/v1/                     # bootstrap config schema
    ├── crud/v1/                     # CRUD framework errors and page-token payload
    ├── errors/v1/                   # error-code annotation extensions
    ├── redact/v3/                   # field-level log-redaction annotations
    ├── contrib/db/redis/v1/         # database section schema
    ├── security/tls/v1/             # shared TLS configuration
    └── transport/http/cors/v1/      # transport config schema

`buf.yaml`、`buf.lock`、`buf.go.gen.yaml` 都在仓库根。imports 相对于 `api/protos/`，例如：

```proto
import "servora/conf/v1/annotations.proto";
```

## 命名与生成约束

- `package` 必须以 `servora.` 开头并带版本后缀，例如 `servora.core.v1`。
- 目录必须与 package 对齐，满足 Buf `PACKAGE_DIRECTORY_MATCH`。
- `go_package` 使用 `github.com/Servora-Kit/servora/api/gen/go/servora/<ns>/v1;<alias>`。
- 新 annotation extension 号段遵守根 `AGENTS.md` 的 `5xx00` 规划。
- 第一方 backend/section 配置 proto 使用 `servora.conf.v1.section` / `field` 表达 section、默认值和必填项。
- core `bootstrap.proto` 内的配置 message 采用所属域名称；TLS、Redis 等配置放在所属域 proto，由 core 引用或业务显式 `bootstrap.Scan`。
- 运行期配置按 owner 归入 `contrib`、`security`、`transport` 或 `obs`。

## 生成与校验

```bash
just lint-proto
just fmt-proto
just gen
just gen-fresh   # 删除/重命名 proto 或移除 plugin 时使用
just bsr-update
just bsr-push
```

修改 proto 后检查 `api/gen/go` diff。生成代码只由 `just gen`/Buf 写入，不手改。

## 常见反模式

- 把业务仓库 service proto 放进本目录。
- 在本目录新增 `buf.yaml` 与根 workspace 分叉。
- import 使用相对 `../` 路径或 generated Go 路径。
- 新增 proto 后忘记同步 `README.md` 中面向 BSR 的说明。

# AGENTS.md - api/protos/

<!-- Parent: ../../AGENTS.md -->
<!-- Updated: 2026-05-24 -->

## 当前定位

`api/protos/` 是 Servora 框架公共 proto contract 根，随仓库根 `buf.yaml` 发布到 `buf.build/servora/servora`。

这里定义框架级 annotation、配置 schema、CloudEvents/audit schema 与少量通用数据结构；不存放业务服务 proto。

## 当前结构

```text
api/protos/
├── README.md
├── AGENTS.md
└── servora/
    ├── audit/v1/                    # audit annotation extensions
    ├── authn/v1/                    # authn annotation extensions
    ├── authz/v1/                    # authz annotation + runtime types
    ├── cloudevents/v1/              # CloudEvents envelope schema
    ├── conf/v1/                     # config annotation extensions
    ├── core/v1/                     # Bootstrap config schema
    ├── extra/{audit,broker,cors,jwt,mail}/v1/
    ├── mapper/v1/                   # mapper annotation extensions
    ├── pagination/v1/               # pagination public types
    └── security/auth{n,z}/.../v1/    # security backend config schema
```

`buf.yaml`、`buf.lock`、`buf.go.gen.yaml` 都在仓库根。imports 相对于 `api/protos/`，例如：

```proto
import "servora/audit/v1/annotations.proto";
```

## 命名与生成约束

- `package` 必须以 `servora.` 开头并带版本后缀，例如 `servora.core.v1`。
- 目录必须与 package 对齐，满足 Buf `PACKAGE_DIRECTORY_MATCH`。
- `go_package` 使用 `github.com/Servora-Kit/servora/api/gen/go/servora/<ns>/v1;<alias>`。
- 新 annotation extension 号段遵守根 `AGENTS.md` 的 `5xx00` 规划。
- `service_default` 合并语义必须与生成器测试一致：方法级显式字段覆盖服务级默认，未显式字段继承。
- 第一方 backend 配置 proto 必须用 `servora.conf.v1.section` / `field` 表达 section、默认值和必填项；不要让 runtime 包重复维护默认值或 required 判断。

## 生成与校验

```bash
make lint.proto
make fmt.proto
make gen
make gen.fresh   # 删除/重命名 proto 或移除 plugin 时使用
make bsr.update
make bsr.push
```

修改 proto 后检查 `api/gen/go` diff。生成代码只由 `make gen`/Buf 写入，不手改。

## 常见反模式

- 把业务仓库 service proto 放进本目录。
- 在本目录新增 `buf.yaml` 与根 workspace 分叉。
- import 使用相对 `../` 路径或 generated Go 路径。
- 新增 proto 后忘记同步 `README.md` 中面向 BSR 的说明。

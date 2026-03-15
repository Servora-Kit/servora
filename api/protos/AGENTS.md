# AGENTS.md - api/protos/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 当前定位

`api/protos/` 现在是 **共享 proto 模块**，不再承载全部服务协议。当前目录真实内容：
- `conf/`：配置结构 proto
- `pagination/`：分页相关公共 proto
- `template/`：`svr new api` 命令使用的 proto 模板

服务专属协议已拆到：
- `app/iam/service/api/protos/`
- `app/sayhello/service/api/protos/`

## 当前结构

```text
api/protos/
├── AGENTS.md
├── buf.yaml
├── buf.lock
├── conf/
└── pagination/
```

## 目录说明

- `conf/v1/conf.proto`：服务配置映射
- `pagination/`：分页请求/响应的公共定义
- `template/service/v1/`：`svr new api` 命令使用的 proto 模板（`template.proto` 与 `template_doc.proto`）

## 生成与校验

在项目根目录执行：

```bash
make gen
```

只校验共享 proto 模块时：

```bash
cd api/protos && buf lint
cd api/protos && buf format -w
cd api/protos && buf mod update
```

## 维护提示

- 这里不再列出 `auth/`、`user/`、`servora/`、`sayhello/` 等业务目录，那些协议已不在本模块
- 若要更新 IAM 或 sayhello 的实际接口，请改对应服务目录下的 `api/protos/`
- `template/` 目录仅用于 `svr new api` 命令，不参与实际代码生成

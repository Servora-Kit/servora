## Why

服务 proto 已经迁移到各服务自己的 `app/<service>/service/api/protos/` 目录，手动新建目录、proto 文件和 doc proto 的重复操作又回来了。与其继续使用已经下线的共享目录脚手架，不如恢复一个面向服务目录的 `svr new api`，让新建 gRPC 协议时仍有统一入口。

## What Changes

- 恢复 `svr new api` 命令，但将签名调整为 `svr new api <name> <server_name>`。
- 命令默认只生成服务内 gRPC proto 骨架，不生成 HTTP 专用 `i_*.proto`。
- 生成目标从旧的 `api/protos/` 改为 `app/<server_name>/service/api/protos/<name>/service/v1/`。
- CLI 需要校验目标服务存在，并与当前 Buf v2 workspace、服务级 OpenAPI / TS 生成约定保持一致。
- 更新 README、AGENTS 与 OpenSpec，重新把该命令定义为受支持能力。

## Capabilities

### New Capabilities
- `svr-new-api-command`: 提供面向服务目录的 gRPC proto 脚手架命令，支持 `svr new api <name> <server_name>`。

### Modified Capabilities
- `cli-ux-enhancement`: CLI 命令集合从仅保留 `svr gen gorm` 调整为重新包含 `svr new api`，并明确其服务维度输入语义。

## Impact

- 受影响代码：`cmd/svr` 命令注册与新建 proto 的实现逻辑
- 受影响目录：`app/*/service/api/protos/`、相关模板目录与文档
- 受影响文档：`README.md`、`AGENTS.md`、`cmd/svr/AGENTS.md`、`api/AGENTS.md`、`api/protos/AGENTS.md`
- 受影响规范：需要新增 `svr-new-api-command` 规范，并调整 `cli-ux-enhancement` 的 CLI 能力描述

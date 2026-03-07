## Why

`svr` CLI 目前只有 `gen` 命令组，缺少脚手架能力。开发者新建一个 gRPC 服务时，需要手动创建 proto 目录结构和文件，容易遗漏约定或出现格式不一致。通过 `svr new api <name>` 命令，可以一键生成符合 servora 规范的 proto 文件骨架，提升一致性并降低上手成本。

## What Changes

- **新增** `svr new` 命令组（`cmd/svr/internal/cmd/new/`）
- **新增** `svr new api <name>` 子命令，根据名称生成 proto 脚手架
- **新增** embed 内嵌默认模板（`cmd/svr/internal/cmd/new/template/protos/`）
- **新增** 项目级可覆盖模板（`api/protos/template/service/v1/`）
- **修改** `cmd/svr/internal/root/root.go` 注册 `new` 命令组
- **修改** `AGENTS.md` 补充 `svr` 命令须在项目根目录执行的约定

## Capabilities

### New Capabilities

- `svr-new-api`: `svr new api` 子命令——解析用户输入的服务名、查找模板、执行命名替换、将 proto 文件写入目标目录

### Modified Capabilities

（无规范级行为变更）

## Impact

- `cmd/svr/` — 新增 `new` 命令组及实现文件
- `api/protos/template/` — 新增项目级默认模板目录（演示用，可被用户修改）
- `AGENTS.md` — 补充 `svr` 执行目录约定
- 不影响现有 `gen` 命令，不引入破坏性变更

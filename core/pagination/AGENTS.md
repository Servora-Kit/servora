# AGENTS.md - core/pagination/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供分页请求/响应的轻量辅助函数，围绕共享 proto 类型统一页码、页大小与响应构造逻辑。

## 当前文件

- `helpers.go`：`ExtractPage`、`BuildPageResponse` 与默认值定义

## 当前实现事实

- 默认页码为 `1`
- 默认页大小为 `20`
- 依赖共享生成类型 `api/gen/go/servora/pagination/v1`
- 包体量很小，定位是“分页辅助”，不是通用查询框架

## 边界约束

- 这里只处理分页元数据，不负责数据库查询、排序、过滤或游标协议
- 不在这里扩展业务列表接口语义
- 不把分页 helper 与具体 ORM / repository 实现耦合

## 常见反模式

- 把 SQL 查询拼装放进 `core/pagination`
- 同时支持多种彼此冲突的分页语义，导致 helper 失去简单性
- 在各服务里重复硬编码默认页码/页大小而绕过共享 helper

## 测试与使用

```bash
go test ./core/pagination/...
```

## 维护提示

- 若调整默认值，需同步确认前端与各服务接口契约是否接受
- 若 proto 分页类型有变更，先执行根目录 `make gen`

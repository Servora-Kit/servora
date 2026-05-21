# AGENTS.md - transport/server/http/swagger/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 模块目的

提供 Swagger UI 与 OpenAPI 文档挂载能力，统一把生成好的 spec 以固定入口暴露给 HTTP 服务。

## 当前文件

- `swagger.go`：注册路由与 handler 的主入口
- `swagger-ui.html`：UI 模板

## 当前实现事实

- `Register()` 负责挂载 `{basePath}/` 与 `{basePath}/openapi.yaml`
- 默认标题为 `API Documentation`
- 默认 base path 为 `/docs/`
- 该包关注的是“如何暴露文档”，不是“如何生成 OpenAPI”

## 边界约束

- OpenAPI 产物由调用方准备；本包只负责把已有 spec 暴露给 HTTP 服务
- 不在这里加入业务鉴权、API 网关或静态站点编排逻辑
- 不把 UI 模板与服务私有 branding 深度耦合到共享包

## 常见反模式

- 在 `transport/server/http/swagger` 中加入 OpenAPI 生成逻辑，混淆挂载与生成职责
- 多处手写 `/docs` 路由而绕过统一注册入口
- 把共享 UI 模板改成某个服务专用页面，影响其他服务复用

## 测试与使用

```bash
go test ./transport/server/http/swagger/...
```

## 维护提示

- 若调整默认路由或模板结构，需同步确认各服务文档访问入口是否仍稳定
- 若变更静态资源引用方式，优先保证离线本地运行仍可用

# AGENTS.md - web/packages/proto-utils/src/crud/

<!-- Parent: ../../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 模块目的

`crud` 是 `@servora/proto-utils/crud` 的框架无关 TypeScript 运行时，供 `protoc-gen-servora-crud target=ts` 生成物消费。

## 核心边界

- 只提供资源名错误、FieldMask、AIP filter/order 文本和无状态 pager helper。
- 不生成或持有 Proto message、HTTP client、UI hooks、业务状态、授权或持久化逻辑。
- 所有 helper 不修改调用方传入对象；字段参数必须来自生成的 `XxxFields` / `XxxUpdateFields`。
- Resource name 只处理未 URL 编码 canonical name；百分号编码属于 transport。

## 验证

在 `servora/` 执行：

```bash
make web.typecheck
make web.build
```

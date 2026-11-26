# AGENTS.md - web/packages/proto-utils/test/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-07-22 -->

## 当前定位

`test/` 保存 `@servora/proto-utils` 已构建公开入口的 Node 合同测试，覆盖传输编码、错误合同和包 exports；不再测试公共 HTTP request handler。

## 核心约束

- 测试只断言公开导出和可观察行为，不读取源码文本或依赖实现私有结构。
- 测试必须确定性、无网络、无外部服务，并可在默认 CI 中运行。
- 新增公开 TypeScript 合同时同步 package test script 与类型检查。

## 验证

在 `servora/web/packages/proto-utils/` 执行：

```bash
pnpm test
pnpm typecheck
```

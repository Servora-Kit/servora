# AGENTS.md - obs/audit/

<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-03-22 | Updated: 2026-03-22 -->

## 模块目的

提供审计事件运行时能力，围绕 `Event`、`Emitter`、`Recorder` 与 middleware 骨架组织统一的审计记录链路。

## 当前文件

- `event.go`：审计事件模型
- `emitter.go`：`Emitter` 接口定义
- `recorder.go`：记录器抽象
- `broker_emitter.go`：基于消息代理的 emitter
- `log_emitter.go`：基于日志输出的 emitter
- `noop_emitter.go`：空实现 emitter
- `middleware.go`：Kratos middleware 骨架
- `proto.go`：proto 相关转换辅助
- `config.go`：配置装配辅助

## 当前实现事实

- `Emitter` 暴露 `Emit(ctx, event)` 与 `Close()` 生命周期接口
- emit 失败不应影响主业务流程，审计属于旁路能力而非交易主路径
- 当前 `middleware.go` 更偏骨架与占位，完整审计编排通常仍需业务侧补充上下文
- 包内同时提供 broker / log / noop 多种 emitter，以适配不同部署形态

## 边界约束

- 本包负责“记录审计事件”，不负责认证、授权、风控或业务补偿
- 不在这里强行定义具体业务事件枚举；业务语义由调用方决定
- 不把审计失败升级为会中断请求的致命错误

## 常见反模式

- 审计发送失败后直接返回 5xx 或 `panic`
- 在 audit 包中塞入具体业务资源模型与领域判断
- 把 middleware 当作唯一入口，忽略 recorder / emitter 的独立可组合性

## 测试与使用

```bash
go test ./obs/audit/...
```

## 维护提示

- 若新增 emitter 类型，优先保持 `Emitter` 接口最小稳定，不要把后端细节泄漏到调用方
- 若补全 middleware，请保持“失败不影响主流程”的默认原则

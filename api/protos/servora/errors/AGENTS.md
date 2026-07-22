# AGENTS.md - api/protos/servora/errors/

<!-- Parent: ../../AGENTS.md -->
<!-- Updated: 2026-07-21 -->

## 当前定位

`api/protos/servora/errors/` 定义 Servora 公共错误码注解，由仓库内同名 `protoc-gen-go-errors` 消费。

注解与使用它们的业务/框架错误枚举同属 `buf.build/servora/servora`，不依赖 Kratos BSR schema；生成的 constructor 使用 Kratos v3 errors runtime。

## 当前结构

```text
errors/
├── AGENTS.md
└── v1/
    └── errors.proto  # EnumOptions default_code 与 EnumValueOptions code
```

## 约束

- extension 号固定为 `50500`（enum default）和 `50501`（enum value override）。
- 注解只描述 HTTP code；reason 仍来自枚举值名，message 来自调用点。
- 新增 option 必须同步仓库内 `cmd/protoc-gen-go-errors` 测试与根 AGENTS 号段表。
- 不在此处定义业务错误枚举。

## 验证

在 `servora/` 执行：

```bash
make lint.proto
go test ./cmd/protoc-gen-go-errors
make gen
```

# Business Log Ctx Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 servora 框架层面提供清晰的 ctx 绑定规范文档与可运行示例，并把 servora-platform/audit data 层的现有 `r.log.{Info,Warn,Error}f(...)` 调用全部迁移为 `r.log.WithContext(ctx).{Info,Warn,Error}f(...)`，让所有业务日志自动带上 OTel trace_id/span_id。

**Architecture:** 三段式落地——(1) 框架侧：在 `obs/logging` 加 godoc Example test 作为可运行规范，明确 `*log.Helper.WithContext(ctx)` 是激活 valuer 的唯一姿势；(2) 文档侧：在 `servora/AGENTS.md` 增设「日志规范」章节强约束；(3) 业务侧：以 servora-platform/audit 的 data 层为示范完成迁移，并通过 e2e curl + grep 验证 trace_id 真实出现。worker_client.go 中的实验性 trace-test 日志同步清理。

**Tech Stack:**
- `github.com/go-kratos/kratos/v2/log` — `Helper.WithContext` / `Valuer`
- `go.uber.org/zap/zaptest/observer` — example test 中验证 valuer 求值
- `curl` + `grep` — e2e 日志验证

---

## File Structure

| 文件 | 操作 | 责任 |
|---|---|---|
| `servora/obs/logging/example_test.go` | Create | godoc Example，演示 `*Helper.WithContext(ctx)` 与 valuer 交互的对照（无 ctx → 空，有 ctx → 求值） |
| `servora/obs/logging/log.go` | Modify | 在 `For` / `NewHelper` godoc 加 ctx 绑定提示，引用 example |
| `servora/AGENTS.md` | Modify | 新增「日志规范」章节，明文约束 data/biz 层必须 `WithContext(ctx)` |
| `servora-platform/app/audit/service/internal/data/audit.go` | Modify | 迁移 2 处 `r.log.Warnf` → `r.log.WithContext(ctx).Warnf` |
| `servora-platform/app/audit/service/internal/data/batch_writer.go` | Modify | 迁移 5 处 `w.log.Warnf` → `w.log.WithContext(ctx).Warnf` |
| `servora-platform/app/audit/service/internal/data/consumer.go` | Modify | 迁移 3 处 `c.log.Warnf` → `c.log.WithContext(ctx).Warnf`（区分有/无 ctx 的位置） |
| `servora-example/app/master/service/internal/data/worker_client.go` | Modify | 删除 `[trace-test-with-ctx]` 实验日志行 |

---

## Task 1: 写框架侧 Example test 演示 WithContext 行为

**Files:**
- Create: `servora/obs/logging/example_test.go`

- [ ] **Step 1: 写失败 Example test — 对照组（无 ctx）vs 治疗组（带 ctx）**

```go
// servora/obs/logging/example_test.go
package logger_test

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// captureLogger records each Log() invocation as a flat key-value map for
// inspection. It mimics how Kratos resolves Valuer fields at log-emit time.
type captureLogger struct {
	captured []map[string]any
}

func (c *captureLogger) Log(level log.Level, keyvals ...any) error {
	m := make(map[string]any, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		m[fmt.Sprint(keyvals[i])] = keyvals[i+1]
	}
	c.captured = append(c.captured, m)
	return nil
}

// ExampleHelper_WithContext demonstrates the only correct way to make a
// kratos Valuer pull values out of the request context. Without
// WithContext(ctx), the valuer is called with context.Background() and yields
// the zero value.
func ExampleHelper_WithContext() {
	type ctxKey string
	const userIDKey ctxKey = "user_id"

	// Valuer extracts a string from ctx; returns "" when key is absent.
	userIDValuer := log.Valuer(func(ctx context.Context) any {
		if v, ok := ctx.Value(userIDKey).(string); ok {
			return v
		}
		return ""
	})

	capture := &captureLogger{}
	base := log.With(capture, "user_id", userIDValuer)
	helper := log.NewHelper(base)

	// Wrong: no WithContext → valuer sees context.Background() → empty.
	helper.Info("call without ctx")

	// Right: WithContext(ctx) → valuer sees the request ctx → resolved.
	ctx := context.WithValue(context.Background(), userIDKey, "alice")
	helper.WithContext(ctx).Info("call with ctx")

	fmt.Printf("without ctx: user_id=%q\n", capture.captured[0]["user_id"])
	fmt.Printf("with ctx:    user_id=%q\n", capture.captured[1]["user_id"])
	// Output:
	// without ctx: user_id=""
	// with ctx:    user_id="alice"
}
```

- [ ] **Step 2: 跑 example test 确认通过**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./obs/logging/ -run ExampleHelper_WithContext -v`
Expected: PASS（Example 的 stdout 与 `// Output:` 块字面匹配）

- [ ] **Step 3: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
git add obs/logging/example_test.go
git commit -m "test(obs/logging): add Example demonstrating Helper.WithContext valuer activation"
```

---

## Task 2: 修订 logger.For / NewHelper godoc

**Files:**
- Modify: `servora/obs/logging/log.go`

- [ ] **Step 1: 替换 `For` 函数上方注释**

定位 `servora/obs/logging/log.go:196-200`（`// For creates...` 至 `// After: ...`），替换为：

```go
// For creates a *Helper scoped to the given module — the one-liner replacement
// for logger.NewHelper(l, logger.WithModule("x/y/z")).
//
//	Before: logger.NewHelper(l, logger.WithModule("user/biz/iam-service"))
//	After:  logger.For(l, "user/biz/iam")
//
// IMPORTANT: The returned helper is NOT bound to any context. To make
// Kratos valuers (such as trace_id / span_id from tracing.Server middleware)
// resolve at log-emit time, callers MUST chain .WithContext(ctx) per call:
//
//	r.log.WithContext(ctx).Warnf("query failed: %v", err)
//
// Calling helper methods directly (r.log.Warnf(...)) without .WithContext
// will leave context-derived fields blank. See ExampleHelper_WithContext.
```

- [ ] **Step 2: 替换 `NewHelper` 函数上方注释**

定位 `servora/obs/logging/log.go:204-205`（`// NewHelper creates a *Helper...` 至 `// Prefer For() when only a module label is needed.`），替换为：

```go
// NewHelper creates a *Helper with optional Option fields applied.
// Prefer For() when only a module label is needed.
//
// IMPORTANT: As with For(), callers must chain .WithContext(ctx) at each
// call site to activate context-aware valuers (trace_id, span_id, etc).
// See ExampleHelper_WithContext for the correct pattern.
```

- [ ] **Step 3: 验证 godoc 渲染无误 + 编译**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go build ./obs/logging/ && go doc ./obs/logging/ For`
Expected: godoc 输出包含 `IMPORTANT: The returned helper is NOT bound to any context`

- [ ] **Step 4: 提交**

```bash
git add obs/logging/log.go
git commit -m "docs(obs/logging): clarify WithContext requirement on For/NewHelper"
```

---

## Task 3: 在 servora/AGENTS.md 加「日志规范」章节

**Files:**
- Modify: `servora/AGENTS.md`

- [ ] **Step 1: 在「常用命令」上方插入「日志规范」小节**

定位 `servora/AGENTS.md` 中 `## 常用命令` 一行，**在其上方** 插入：

````markdown
## 日志规范

### Trace 关联（强制）

业务代码（`internal/data/`、`internal/biz/`、`internal/service/`）中所有 helper 日志调用 **必须** 链式 `WithContext(ctx)`，否则 trace_id / span_id 字段会输出空字符串：

```go
// ❌ 错误 — trace_id 为空，无法关联到 access log
r.log.Warnf("query failed: %v", err)

// ✅ 正确 — trace_id 自动从 ctx 抽取
r.log.WithContext(ctx).Warnf("query failed: %v", err)
```

适用范围：

| 层 | 是否必须 | 说明 |
|---|---|---|
| `internal/data/`（DB / Redis / Kafka / HTTP client） | ✅ 必须 | 慢/失败排障关键点 |
| `internal/biz/`（业务事件） | ✅ 必须 | 线上事件查证刚需 |
| `internal/service/` | ⚠️ 建议 | Kratos `logging.Server` 中间件已为每次 RPC 打 access log |
| `cmd/server/main.go` 等启动路径 | ❌ 不需要 | 无活跃 span，加了也是空 |

### 例外

- 无 ctx 可用时（如后台 ticker、独立 goroutine 起点）：使用裸 helper 是可接受的，但应在日志体里显式注明 `goroutine=ticker` 等可识别字段
- bootstrap 阶段日志：valuer 输出空串属正常行为，可考虑全局 logger 不挂 trace valuer

### 参考

- 框架侧示例：`obs/logging/example_test.go` 中的 `ExampleHelper_WithContext`
- 实证验证：`docs/superpowers/plans/2026-04-25-business-log-ctx-binding.md` Task 4-5
````

- [ ] **Step 2: 验证字符串确实写入**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && grep -c "## 日志规范" AGENTS.md`
Expected: `1`

- [ ] **Step 3: 提交**

```bash
git add AGENTS.md
git commit -m "docs(repo): add 日志规范 section enforcing WithContext on business helpers"
```

---

## Task 4: 迁移 audit.go 并 e2e 验证 trace_id 真出现

**Files:**
- Modify: `servora-platform/app/audit/service/internal/data/audit.go`

> **前置**：本 Task 在 `servora-platform` 仓库内进行。需要先 `cd /Users/horonlee/projects/go/servora-kit/servora-platform`。需要 Kafka + ClickHouse 已在跑（`make compose.up.infra`）以便 e2e 验证。

- [ ] **Step 1: 拍快照——当前 `r.log.Warnf` 处的日志 trace_id 必为空**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
grep -n "r.log\." app/audit/service/internal/data/audit.go
```
Expected output:
```
99:		r.log.Warnf("failed to close rows: %v", closeErr)
123:		r.log.Warnf("failed to scan row: %v", err)
```

- [ ] **Step 2: 修改 L99 — 加 WithContext(ctx)**

Edit `app/audit/service/internal/data/audit.go` L99：

替换：
```go
r.log.Warnf("failed to close rows: %v", closeErr)
```
为：
```go
r.log.WithContext(ctx).Warnf("failed to close rows: %v", closeErr)
```

> 注意：L99 在 `defer func() { ... }()` 内，闭包捕获 `Query` 调用入参 `ctx`，可直接闭包引用同名 `ctx` 变量。

- [ ] **Step 3: 修改 L123 — 加 WithContext(ctx)**

替换：
```go
r.log.Warnf("failed to scan row: %v", err)
```
为：
```go
r.log.WithContext(ctx).Warnf("failed to scan row: %v", err)
```

- [ ] **Step 4: 验证迁移完整性 — grep 不应再出现裸调用**

Run: `grep -n "r\.log\.\(Info\|Warn\|Error\)" app/audit/service/internal/data/audit.go | grep -v WithContext`
Expected: 无输出

- [ ] **Step 5: 编译**

Run: `cd app/audit/service && go build ./...`
Expected: 无错误

- [ ] **Step 6: e2e 验证 — 重启服务 + 触发请求**

```bash
# 终端 A：重启 audit 服务（air 自动热加载，或 kill + re-run）
cd app/audit/service && make dev

# 终端 B：触发 list 请求
rtk curl -s "http://localhost:8000/v1/audit/events?page_size=1"
```

观察终端 A 日志：
- 必有的：`module=audit/server/http` access log，`trace_id` 非空
- close 失败路径不易触发，本 Task 仅靠 grep 完整性 + 编译验证；高频路径在 Task 5 验证

- [ ] **Step 7: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
git add app/audit/service/internal/data/audit.go
git commit -m "refactor(audit/data): bind ctx to log helper for trace correlation"
```

---

## Task 5: 批量迁移 batch_writer.go + consumer.go

**Files:**
- Modify: `servora-platform/app/audit/service/internal/data/batch_writer.go`
- Modify: `servora-platform/app/audit/service/internal/data/consumer.go`

- [ ] **Step 1: 列出 batch_writer.go 所有需迁移的调用**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-platform && grep -n "w\.log\." app/audit/service/internal/data/batch_writer.go`
Expected output:
```
133:		w.log.Warnf("failed to prepare batch: %v", err)
186:			w.log.Warnf("append failed for event %s, aborting batch: %v", e.EventId, err)
194:		w.log.Warnf("failed to send batch: %v", err)
207:				w.log.Warnf("failed to ack event: %v", err)
217:				w.log.Warnf("failed to nack event: %v", err)
```

- [ ] **Step 2: 检查每处所在函数是否带 ctx 入参**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-platform && grep -n "^func.*batch\|^func.*flush\|^func.*ack\|^func.*Run" app/audit/service/internal/data/batch_writer.go`
Expected: 函数定义中含 `ctx context.Context` 参数。如某方法不带 ctx（典型如 background goroutine 顶层），按规范文档「例外」处理。

- [ ] **Step 3: 替换 batch_writer.go 中的 5 处调用**

对 L133 / L186 / L194 / L207 / L217 各处做相同模式替换：
```go
w.log.Warnf(...)  →  w.log.WithContext(ctx).Warnf(...)
```

如某行所在函数无 ctx 入参且无法获取（如 background goroutine 顶层），保留原状并在该行上方加注释：
```go
// goroutine root: no request ctx available
w.log.Warnf(...)
```

- [ ] **Step 4: 列出 consumer.go 所有调用**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-platform && grep -n "c\.log\." app/audit/service/internal/data/consumer.go`
Expected output:
```
80:			c.log.Warnf("failed to unsubscribe: %v", err)
98:		c.log.Warnf("failed to unmarshal audit event: %v", err)
104:		c.log.Warnf("invalid audit event: %v", err)
```

- [ ] **Step 5: 替换 consumer.go 中的 3 处**

- L80（`unsubscribe` 失败，发生在关闭路径）：检查所在函数是否带 ctx；若带则加 WithContext，若不带（如 close 钩子）则按「例外」处理保留原状
- L98、L104（消息消费循环内）：必带 ctx，加 WithContext

- [ ] **Step 6: grep 校验剩余裸调用**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-platform && grep -n "\(w\|c\|r\)\.log\.\(Info\|Warn\|Error\)" app/audit/service/internal/data/*.go | grep -v WithContext | grep -v "// goroutine root"`
Expected: 无输出

- [ ] **Step 7: 编译 + 测试**

Run: `cd app/audit/service && go build ./... && go test -race ./internal/data/...`
Expected: 编译通过、测试通过

- [ ] **Step 8: e2e 验证 — 触发 consumer 路径**

```bash
# 重启 audit 服务（make dev 应已跑着，热重载会自己处理；如 kafka 离线，手动重启）

# 触发请求 + 观察 audit consumer 日志
rtk curl -s "http://localhost:8000/v1/audit/events?page_size=1"
```

期望：在 audit 服务终端中，`module=core/data/audit` 类的日志若被触发（如 batch flush 失败），日志行 JSON 含 `"trace_id":"<32 hex>"` 而非 `"trace_id":""`。

如能向 Kafka 投递无效 message 触发 L98 路径更佳；若无投递工具，临时停掉 ClickHouse 容器制造写入失败可触发 L186/L194。

- [ ] **Step 9: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
git add app/audit/service/internal/data/batch_writer.go app/audit/service/internal/data/consumer.go
git commit -m "refactor(audit/data): bind ctx to log helpers in batch_writer and consumer"
```

---

## Task 6: 清理 servora-example 实验残留

**Files:**
- Modify: `servora-example/app/master/service/internal/data/worker_client.go`

- [ ] **Step 1: 定位实验代码**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-example && grep -n "trace-test" app/master/service/internal/data/worker_client.go`
Expected output:
```
44:	c.log.WithContext(ctx).Info("[trace-test-with-ctx] worker hello called")
```

- [ ] **Step 2: 删除该行**

Edit `app/master/service/internal/data/worker_client.go` L44，删除整行：
```go
c.log.WithContext(ctx).Info("[trace-test-with-ctx] worker hello called")
```

- [ ] **Step 3: 验证已清理**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-example && grep -c "trace-test" app/master/service/internal/data/worker_client.go`
Expected: `0`

- [ ] **Step 4: 编译 + lint**

Run: `cd app/master/service && go build ./... && cd ../../.. && make lint.go`
Expected: 无错误

- [ ] **Step 5: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-example
git add app/master/service/internal/data/worker_client.go
git commit -m "chore(master/data): remove trace-correlation experiment log line"
```

---

## Task 7: 全量回归

**Files:** 无

- [ ] **Step 1: servora 仓库测试 + lint**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
go test -race ./obs/logging/...
make lint
```
Expected: All PASS

- [ ] **Step 2: servora-platform 仓库测试 + lint**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
go test -race ./app/audit/service/internal/data/...
make lint
```
Expected: All PASS

- [ ] **Step 3: servora-example 仓库测试 + lint**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-example
go test -race ./app/master/service/internal/data/...
make lint.go
```
Expected: All PASS

- [ ] **Step 4: e2e 链路验证 — 同一次请求的 trace_id 在多层日志中一致**

```bash
# 重启 audit 服务
cd /Users/horonlee/projects/go/servora-kit/servora-platform/app/audit/service && make dev &

# 触发请求
rtk curl -s "http://localhost:8000/v1/audit/events?page_size=1"
```

观察日志：
- `module=audit/server/http` 这条 access log 带 `trace_id="X"`
- 若触发了 data 层日志（如 close 失败、batch flush 失败），其 `trace_id` 也应是 `"X"`

将 trace_id 一致性观察记录到 PR 描述。

---

## Task 8: 关闭 TODO + 打 tag

**Files:**
- Modify: `servora/docs/TODO.md`

- [ ] **Step 1: 把 P2-1b 移到「已完成」段**

Edit `servora/docs/TODO.md`：

定位 `## 已完成` 段，追加（如已有 P2-1a 完成条目则在其下追加）：

```markdown
### [P2-1b] 业务侧日志 ctx 绑定规范 ✅ 2026-MM-DD

framework 侧：`obs/logging/example_test.go` 提供可运行示例，`servora/AGENTS.md` 增设「日志规范」章节强约束。
业务侧：servora-platform/audit data 层 10 处 helper 调用全部迁移为 `WithContext(ctx)`，trace_id 在 access log 与 data 层日志间贯通。
详见 `docs/superpowers/plans/2026-04-25-business-log-ctx-binding.md`。
```

并删除 P2 段中的 P2-1b 条目（如尚未拆分则按 plan 7 中的 P2-1a/b 拆分一并处理）。

- [ ] **Step 2: 提交（servora 仓库）**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
git add docs/TODO.md
git commit -m "docs(repo): close P2-1b — business log ctx binding shipped"
```

- [ ] **Step 3: 视情况打 tag**

按 `servora/AGENTS.md` 规则：本 plan 主要修改 `obs/logging/log.go`（公共 API godoc 修订）+ `AGENTS.md` + `docs/`。其中 godoc 修订对 import 此包的业务有提示价值，建议打补丁版 tag。

```bash
make tag TAG=v0.x.y  # 替换为下一个补丁版本号
```

> 如与 P2-1a 一起打包发布，按更高版本号一起打 tag，避免 churn。

---

## Self-Review Checklist

- [x] 每个 task 都有可执行的具体动作（grep / edit / run / commit）
- [x] 所有 grep 命令含 expected output，便于断言
- [x] 跨仓库操作（servora / servora-platform / servora-example）的 `cd` 路径每次明确
- [x] 例外情况（无 ctx 可用的 goroutine 起点）有明确处理建议
- [x] e2e 验证手段（curl + grep）具体可重复
- [x] Task 之间符号一致：`WithContext(ctx)` 全文统一，无 `WithCtx` 等变体
- [x] Task 6 清理实验残留与本 plan 主线相关（避免遗留示例代码混淆 PR review）

## Risks & Mitigations

| 风险 | 缓解 |
|---|---|
| Kratos `*log.Helper.WithContext(ctx)` 是否真的返回新 helper（不修改原 receiver） | Task 1 example test 通过对照组（先调一次无 ctx）+ 治疗组（再调一次带 ctx）证明原 helper 状态未变 |
| 业务代码中存在 close hook / background goroutine 没有 ctx 可用 | 规范文档明确「例外」处理：保留裸调用 + 行内注释 `// goroutine root` |
| WithContext 链式调用每次创建新 helper，可能有微小性能开销 | Helper 是栈分配的小结构体，开销可忽略；如热路径压测显示问题再考虑 helper 缓存 |
| L99 在 `defer` 内闭包捕获 ctx，理论上 ctx 可能已被 cancel | 这是预期行为；ctx 是否 cancel 不影响 SpanContext 的有效性，trace_id 仍可读 |
| AGENTS.md 修改未被业务团队读到 | 打 tag 时在 release notes 顶部声明 "新增日志规范"，并在 PR template 加 checkbox 「Helper 调用已加 WithContext」 |

## Out of Scope

- 编写 lint 工具自动检测 `r.log.Warnf` 没绑 ctx 的调用点（独立 follow-up，可作为 P3）
- 迁移 servora-example 的 master/worker 业务日志（example 性质，非生产路径）
- biz / service 层日志迁移（本 plan 仅覆盖 data 层；biz/service 层 helper 调用很少，按规范文档自然演进）
- Bootstrap 阶段日志的「invalid SpanContext 时省略字段」优化（属 P2-1d，单独处理）

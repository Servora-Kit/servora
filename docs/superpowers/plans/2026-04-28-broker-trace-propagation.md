# Broker Trace Propagation + Logger Helper Ctx Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 servora Kafka broker plugin 因丢弃 `kgo.Record.Context` 导致的分布式 trace 链路断开；同步在 `obs/logging` 的 godoc 与 Example 中明确 `*Helper.WithContext(ctx)` 是激活 Kratos Valuer 的唯一姿势；并把 servora-platform/audit 的 consumer 处理路径里 3 处 helper 调用接到正确的 ctx 上。

**Architecture:**
1. **Framework P0 fix（最重要）**：`servora/infra/broker/kafka/consumer.go:63-75` 在 `fetches.EachRecord` 内**忽略** `r.Context` —— 然而 `kotel.Tracer.OnFetchRecordBuffered` hook 已经把上游 producer 抽出的 span context 写到 `r.Context`。修复：把 `EachRecord` 内的逻辑抽成 `(s *kafkaSubscriber) dispatch(loopCtx, r)` 方法，handler 收到 `r.Context` 优先、nil 时回落 `loopCtx`。该改动让上游 produce span ↔ 下游 consume span 在 trace 上重新连通。
2. **Framework godoc**：在 `obs/logging/log.go` 的 `For` / `NewHelper` 上方注释中加入「业务调用必须 `WithContext(ctx)` 才能激活 trace_id/span_id valuer」的硬性提示，并配 Example test 作可执行规范。
3. **Business 侧最小同步**：在 broker fix 落地后，把 `servora-platform/app/audit/service/internal/data/consumer.go:91/98/104` 的 3 处 `c.log.Warnf` 升级为 `c.log.WithContext(ctx).Warnf`，让消费侧日志带上现已正确传入的 trace_id。

**Tech Stack:**
- `github.com/twmb/franz-go` + `plugin/kotel` v1.6.0 — broker 端 OTel hook
- `github.com/go-kratos/kratos/v2/log` — Helper.WithContext / Valuer 模型
- `go test` (含 `-race`) — 单测 / 回归测试
- `curl` — e2e 触发

---

## File Structure

| 文件 | 操作 | 责任 |
|---|---|---|
| `servora/infra/broker/kafka/consumer.go` | Modify | 抽出 `dispatch` 方法；handler ctx 取 `r.Context ?? loopCtx` |
| `servora/infra/broker/kafka/consumer_dispatch_test.go` | Create | 回归测试：record ctx 必须传给 handler；nil 时回落 loop ctx |
| `servora/obs/logging/log.go` | Modify | `For` / `NewHelper` godoc 增补 WithContext 硬性提示 |
| `servora/obs/logging/example_test.go` | Create | `ExampleHelper_WithContext`：演示 valuer 在有/无 ctx 时的差异 |
| `servora-platform/app/audit/service/internal/data/consumer.go` | Modify | `handle()` 内 3 处 `c.log.Warnf` → `c.log.WithContext(ctx).Warnf` |
| `servora/docs/TODO.md` | Modify | 关闭 P2-1b（拆分后只保留必要项）；新增 P0-5（broker trace 断链） |

**明确不做**（旧 plan 错误地包含的项）：
- `servora/AGENTS.md` 整章「日志规范」—— 过度规范化，Example test + godoc 已足够
- `batch_writer.go` 5 处 `WithContext` 迁移 —— 该函数运行在后台 ticker goroutine 内，传入的 ctx 是服务器生命期 ctx 且 batch 跨多个上游请求，加 `WithContext` 后 trace_id 仍为空，反而误导
- `audit.go` 2 处迁移 —— 已在前置探查中完成（`r.log.WithContext(ctx).Warnf`），并被 e2e 实证 trace_id 正常出现
- `servora-example/.../worker_client.go` 实验日志清理 —— 与本 plan 无关，独立小 PR

---

## Task 1: 抽出 dispatch 方法（纯重构，零行为改变）

**Files:**
- Modify: `servora/infra/broker/kafka/consumer.go`

- [ ] **Step 1: 读当前 `poll` 方法定位重构区域**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && sed -n '40,77p' infra/broker/kafka/consumer.go`
Expected: 看到 `fetches.EachRecord(func(r *kgo.Record) { ... })` 闭包内含 7 行逻辑（recordToEvent → handler → AutoAck）。

- [ ] **Step 2: 抽出 `dispatch` 方法并保持原行为**

将 `consumer.go` 中 `poll` 方法内的 `EachRecord` 闭包替换为：

```go
		fetches.EachRecord(func(r *kgo.Record) {
			s.dispatch(ctx, r)
		})
```

并在 `poll` 方法之后追加：

```go
// dispatch hands a single fetched record to the user handler.
// Pure refactor: identical behavior to the previous inline closure.
// A subsequent task replaces the loop ctx with the record-bound ctx.
func (s *kafkaSubscriber) dispatch(loopCtx context.Context, r *kgo.Record) {
	event := recordToEvent(r, s.client)
	if err := s.handler(loopCtx, event); err != nil {
		if s.zap != nil {
			s.zap.Warn("kafka handler error", zap.String("topic", r.Topic), zap.Error(err))
		}
		_ = event.Nack()
		return
	}
	if s.sopts.AutoAck {
		_ = event.Ack()
	}
}
```

- [ ] **Step 3: 编译验证零行为改变**

Run: `cd servora && go build ./infra/broker/kafka/...`
Expected: 无错误。

- [ ] **Step 4: 提交（重构独立成一个 commit，便于二分定位）**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
git add infra/broker/kafka/consumer.go
git commit -m "refactor(broker/kafka): extract dispatch method from poll loop"
```

---

## Task 2: 写**失败**的回归测试，证明当前 dispatch 丢弃 r.Context

**Files:**
- Create: `servora/infra/broker/kafka/consumer_dispatch_test.go`

- [ ] **Step 1: 写测试文件**

Create `servora/infra/broker/kafka/consumer_dispatch_test.go`：

```go
package kafka

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/infra/broker"
	"github.com/twmb/franz-go/pkg/kgo"
)

type ctxProbeKey string

const probeKey ctxProbeKey = "probe"

// TestDispatchPrefersRecordContext is the regression test for the broker
// trace-propagation bug: kotel's OnFetchRecordBuffered writes the upstream
// span context into r.Context, but the previous dispatch implementation
// passed the poll-loop ctx instead, severing distributed trace linkage.
func TestDispatchPrefersRecordContext(t *testing.T) {
	var got context.Context
	s := &kafkaSubscriber{
		handler: func(ctx context.Context, _ broker.Event) error {
			got = ctx
			return nil
		},
		sopts: broker.SubscribeOptions{AutoAck: false},
	}

	rec := &kgo.Record{
		Topic:   "t",
		Context: context.WithValue(context.Background(), probeKey, "from-record"),
	}
	loopCtx := context.WithValue(context.Background(), probeKey, "from-loop")

	s.dispatch(loopCtx, rec)

	if got == nil {
		t.Fatal("handler not invoked")
	}
	if v, _ := got.Value(probeKey).(string); v != "from-record" {
		t.Fatalf("handler received %q, want %q (record ctx must win over loop ctx)", v, "from-record")
	}
}

// TestDispatchFallsBackToLoopContext covers the case where kotel hooks are
// disabled or a record is synthesized without a Context — handler must still
// receive a usable ctx.
func TestDispatchFallsBackToLoopContext(t *testing.T) {
	var got context.Context
	s := &kafkaSubscriber{
		handler: func(ctx context.Context, _ broker.Event) error {
			got = ctx
			return nil
		},
		sopts: broker.SubscribeOptions{AutoAck: false},
	}

	rec := &kgo.Record{Topic: "t"} // Context intentionally nil
	loopCtx := context.WithValue(context.Background(), probeKey, "from-loop")

	s.dispatch(loopCtx, rec)

	if got == nil {
		t.Fatal("handler not invoked")
	}
	if v, _ := got.Value(probeKey).(string); v != "from-loop" {
		t.Fatalf("handler received %q, want %q (nil record ctx must fall back to loop ctx)", v, "from-loop")
	}
}
```

- [ ] **Step 2: 跑测试，确认 `TestDispatchPrefersRecordContext` 失败**

Run: `cd servora && go test -race ./infra/broker/kafka/ -run TestDispatch -v`
Expected:
- `TestDispatchPrefersRecordContext` **FAIL**，错误信息形如 `handler received "from-loop", want "from-record"`
- `TestDispatchFallsBackToLoopContext` **PASS**（当前实现碰巧满足 fallback 行为，因为根本不看 r.Context）

> 这条失败结果就是 bug 的实证。不要在此 task 里修代码。

- [ ] **Step 3: 提交红灯测试**

```bash
git add infra/broker/kafka/consumer_dispatch_test.go
git commit -m "test(broker/kafka): add failing regression for r.Context propagation"
```

---

## Task 3: 修 dispatch — handler 收到 r.Context 优先

**Files:**
- Modify: `servora/infra/broker/kafka/consumer.go`

- [ ] **Step 1: 修改 `dispatch` 方法**

将 Task 1 中新增的 `dispatch` 方法替换为：

```go
// dispatch hands a single fetched record to the user handler.
//
// The kotel OnFetchRecordBuffered hook (wired in broker.go) extracts the
// upstream producer's span context from Kafka headers and stores it on
// r.Context. Passing r.Context to the handler keeps distributed trace
// linkage intact across producer → consumer boundaries.
//
// loopCtx is the long-lived poll-loop ctx and is used only when r.Context
// is nil (e.g. when kotel hooks are disabled).
func (s *kafkaSubscriber) dispatch(loopCtx context.Context, r *kgo.Record) {
	event := recordToEvent(r, s.client)
	msgCtx := r.Context
	if msgCtx == nil {
		msgCtx = loopCtx
	}
	if err := s.handler(msgCtx, event); err != nil {
		if s.zap != nil {
			s.zap.Warn("kafka handler error", zap.String("topic", r.Topic), zap.Error(err))
		}
		_ = event.Nack()
		return
	}
	if s.sopts.AutoAck {
		_ = event.Ack()
	}
}
```

- [ ] **Step 2: 跑测试，确认两条测试都 PASS**

Run: `cd servora && go test -race ./infra/broker/kafka/ -run TestDispatch -v`
Expected: 两条测试都 PASS。

- [ ] **Step 3: 跑整个 broker/kafka 包测试，确认无副作用**

Run: `cd servora && go test -race ./infra/broker/...`
Expected: All PASS。

- [ ] **Step 4: 提交修复**

```bash
git add infra/broker/kafka/consumer.go
git commit -m "fix(broker/kafka): propagate record context to handler so kotel-extracted span survives

Previously fetches.EachRecord passed the poll-loop ctx to the handler,
discarding the span context that kotel.OnFetchRecordBuffered writes into
r.Context. This severed distributed trace linkage at the Kafka boundary —
upstream producer spans and downstream consumer-side processing were
unrelated in the trace UI even though kotel was correctly injecting
W3C traceparent headers.

Handler now receives r.Context when present, falling back to the poll-loop
ctx only when kotel hooks are absent.

fixes TODO.md P0-5"
```

---

## Task 4: 写 logger Example test

**Files:**
- Create: `servora/obs/logging/example_test.go`

- [ ] **Step 1: 创建 Example test 文件**

Create `servora/obs/logging/example_test.go`：

```go
package logger_test

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// captureLogger records each Log() invocation as a flat key→value map so
// the example can inspect what valuers actually emitted.
type captureLogger struct {
	captured []map[string]any
}

func (c *captureLogger) Log(_ log.Level, keyvals ...any) error {
	m := make(map[string]any, len(keyvals)/2)
	for i := 0; i+1 < len(keyvals); i += 2 {
		m[fmt.Sprint(keyvals[i])] = keyvals[i+1]
	}
	c.captured = append(c.captured, m)
	return nil
}

// ExampleHelper_WithContext demonstrates the only correct way to make a
// kratos Valuer pull values out of the request context. Without
// WithContext(ctx), the valuer is invoked with context.Background() and
// yields the zero value — which is exactly what causes empty trace_id /
// span_id fields when business code calls helper.Warnf(...) directly.
func ExampleHelper_WithContext() {
	type ctxKey string
	const userIDKey ctxKey = "user_id"

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

- [ ] **Step 2: 跑 Example，确认通过**

Run: `cd servora && go test ./obs/logging/ -run ExampleHelper_WithContext -v`
Expected: PASS。stdout 与 `// Output:` 字面匹配。

- [ ] **Step 3: 提交**

```bash
git add obs/logging/example_test.go
git commit -m "test(obs/logging): add Example demonstrating Helper.WithContext valuer activation"
```

---

## Task 5: 修订 logger.For / NewHelper godoc

**Files:**
- Modify: `servora/obs/logging/log.go`

- [ ] **Step 1: 替换 `For` 上方注释**

定位 `servora/obs/logging/log.go` 中 `// For creates a *Helper scoped...` 至 `func For(`。完整替换块为：

```go
// For creates a *Helper scoped to the given module — the one-liner replacement
// for logger.NewHelper(l, logger.WithModule("x/y/z")).
//
//	Before: logger.NewHelper(l, logger.WithModule("user/biz/iam-service"))
//	After:  logger.For(l, "user/biz/iam")
//
// IMPORTANT: The returned helper is NOT bound to any context. Kratos Valuers
// (such as the trace_id / span_id valuers registered by bootstrap) are only
// invoked with the ctx attached to the helper at log-emit time. To make
// them resolve, callers MUST chain .WithContext(ctx) per call:
//
//	r.log.WithContext(ctx).Warnf("query failed: %v", err)
//
// Calling helper methods directly (r.log.Warnf(...)) without .WithContext
// leaves context-derived fields blank. See ExampleHelper_WithContext.
func For(l Logger, module string) *Helper {
```

- [ ] **Step 2: 替换 `NewHelper` 上方注释**

定位 `// NewHelper creates a *Helper with optional Option fields applied.` 至 `func NewHelper(`。完整替换块为：

```go
// NewHelper creates a *Helper with optional Option fields applied.
// Prefer For() when only a module label is needed.
//
// IMPORTANT: As with For(), callers must chain .WithContext(ctx) at each
// call site to activate context-aware valuers (trace_id, span_id, etc).
// See ExampleHelper_WithContext for the canonical pattern.
func NewHelper(l Logger, opts ...Option) *Helper {
```

- [ ] **Step 3: 校验 godoc 渲染**

Run: `cd servora && go build ./obs/logging/ && go doc ./obs/logging For`
Expected: 输出包含 `IMPORTANT: The returned helper is NOT bound to any context`。

- [ ] **Step 4: 提交**

```bash
git add obs/logging/log.go
git commit -m "docs(obs/logging): clarify WithContext requirement on For and NewHelper"
```

---

## Task 6: 业务侧 — audit consumer.go 3 处迁移（依赖 Task 3）

**Files:**
- Modify: `servora-platform/app/audit/service/internal/data/consumer.go`

> **前置**：Task 3 必须已合并 / `go.work` 已能解析到修后的 servora。否则即便加了 `WithContext`，consumer 拿到的仍是无 span 的 ctx，e2e 验证会失败。

- [ ] **Step 1: 列出待迁移点**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora-platform && grep -n "c\.log\.\(Info\|Warn\|Error\)" app/audit/service/internal/data/consumer.go`
Expected:
```
56:		c.log.Warn("Kafka broker not configured, audit consumer is disabled")
72:	c.log.Infof("subscribed to audit topic: %s (group: %s)", c.topic, c.group)
80:			c.log.Warnf("failed to unsubscribe: %v", err)
91:		c.log.Warn("received nil message, skipping")
98:		c.log.Warnf("failed to unmarshal audit event: %v", err)
104:		c.log.Warnf("invalid audit event: %v", err)
```

仅迁移 **`handle()` 内** 的 3 处（L91/98/104）。其余位于启动 / 关闭路径（L56/72/80），无活跃 span，不动。L42 的 `log.Infof` 是 `NewConsumer` 构造期，Helper 尚未挂载到 receiver，也不在迁移范围。

- [ ] **Step 2: 修改 L91**

替换：
```go
		c.log.Warn("received nil message, skipping")
```
为：
```go
		c.log.WithContext(ctx).Warn("received nil message, skipping")
```

- [ ] **Step 3: 修改 L98**

替换：
```go
		c.log.Warnf("failed to unmarshal audit event: %v", err)
```
为：
```go
		c.log.WithContext(ctx).Warnf("failed to unmarshal audit event: %v", err)
```

- [ ] **Step 4: 修改 L104**

替换：
```go
		c.log.Warnf("invalid audit event: %v", err)
```
为：
```go
		c.log.WithContext(ctx).Warnf("invalid audit event: %v", err)
```

- [ ] **Step 5: 编译 + 测试**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform/app/audit/service
go build ./...
go test -race ./internal/data/...
```
Expected: 全部通过。

- [ ] **Step 6: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
git add app/audit/service/internal/data/consumer.go
git commit -m "refactor(audit/data): bind ctx to log helper in Kafka message handler

After servora broker now propagates r.Context to handler, the consumer's
handle() function can finally surface upstream trace_id in its log lines
by chaining WithContext(ctx).

Startup/shutdown call sites (configured / disabled / subscribed /
unsubscribed) are intentionally left bare since they have no active span.

depends on servora fix for TODO.md P0-5"
```

---

## Task 7: e2e 端到端验证 — 上游 trace_id 贯通到 audit consumer

**Files:** 无（仅运行验证）

- [ ] **Step 1: 启动基础设施 + audit 服务**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
make compose.up.infra
cd app/audit/service && make run &
```

等到日志出现 `[gRPC] server listening on: [::]:8001` 与 `[HTTP] server listening on: [::]:8000`。

- [ ] **Step 2: 制造一条 invalid Kafka 消息触发 L98 / L104 路径**

利用已有的 audit middleware emitter 走真实路径，或最小重现方式：用 `kcat` 投一条非 protobuf bytes：

```bash
echo "not a protobuf" | kcat -P -b localhost:29092 -t servora.audit.events
```

- [ ] **Step 3: 观察 audit 服务日志**

期望见到形如：
```
WARN  ... "trace_id":"<32 hex>", "span_id":"<16 hex>", "module":"consumer/data/audit", "msg":"failed to unmarshal audit event: ..."
```

**关键断言**：`trace_id` 字段非空。这是 Task 3 broker fix + Task 6 业务迁移联合作用的实证。

> 注：上游 producer 必须使用 OTel propagator 注入 traceparent header。如果 invalid 消息是直接通过 kcat 发送（无 traceparent header），那么 kotel 抽出的 span 会是空 —— 此时 trace_id 仍空属预期。建议改从 `servora-example` master 服务发起一条审计事件来验证完整链路（master 通过 audit middleware emit 事件时 Kratos tracing middleware 会注入 traceparent）。

- [ ] **Step 4: 完整链路验证（推荐）**

```bash
# 启 master 服务（servora-example）
cd /Users/horonlee/projects/go/servora-kit/servora-example/app/master/service && make run &

# 触发一个会发审计事件的请求（具体路径以 master proto 为准）
rtk curl -s "http://localhost:<master_http_port>/<some_audited_route>"
```

在三处日志中搜同一 trace_id：
1. master 服务 access log
2. audit 服务 access log（如该请求触发 audit 查询）
3. audit consumer 处理 master 发出的事件时的 batch flush 日志（**不会**带 trace_id，属预期，见 batch_writer 注解）或 invalid 路径日志（带 trace_id，本次修复目标）

记录 trace_id 一致性截图 / 文本到 PR 描述。

- [ ] **Step 5: 关闭服务**

```bash
pkill -f "audit/service/cmd/server" || true
pkill -f "master/service/cmd/server" || true
cd /Users/horonlee/projects/go/servora-kit/servora-platform && make compose.stop
```

---

## Task 8: 更新 TODO.md — 关 P2-1b、新增 P0-5

**Files:**
- Modify: `servora/docs/TODO.md`

- [ ] **Step 1: 在 P0 段尾新增 P0-5**

定位 `### [P0-4] 框架自身缺少 proto 注解端到端示例` 之后、`---\n\n## P1` 分隔线之前的位置，插入：

```markdown
### [P0-5] Kafka broker 丢弃 record ctx，分布式 trace 在消费侧整段断开 ✅ 2026-04-28

- **现状（已修）**：`infra/broker/kafka/consumer.go:63-75` 的 `fetches.EachRecord` 把 `kotel.OnFetchRecordBuffered` 写入 `r.Context` 的上游 span context 直接丢弃，传 poll-loop 的服务器生命期 ctx 给业务 handler。
- **影响（已恢复）**：上游 producer 与下游 consumer 的 span 在 trace 上不连通；audit / 任何 Kafka 消费者的处理路径日志、metrics、子 span 全部看不到上游 trace_id。
- **修复**：抽出 `dispatch(loopCtx, r)` 方法，handler 收到 `r.Context ?? loopCtx`。回归测试见 `infra/broker/kafka/consumer_dispatch_test.go`。详见 [`superpowers/plans/2026-04-28-broker-trace-propagation.md`](superpowers/plans/2026-04-28-broker-trace-propagation.md)。
```

- [ ] **Step 2: 重写 P2-1b 段为收窄完成版（先在原位置标记完成）**

定位现有 `#### [P2-1b] 业务侧日志 ctx 绑定规范` 整段（含 "现状 / 方案 / Plan"），整体替换为：

```markdown
#### [P2-1b] Logger Helper Ctx 绑定规范（godoc + Example）✅ 2026-04-28

`obs/logging/example_test.go` 提供 `ExampleHelper_WithContext` 作为可执行规范；`obs/logging/log.go` 中 `For` / `NewHelper` godoc 增补「必须 `WithContext(ctx)`」硬性提示。业务侧仅迁移 audit `data/audit.go`（2 处）与 `data/consumer.go` `handle()` 内（3 处）—— 这些点 ctx 必含 span，是真实的 trace 关联缺口。`batch_writer.go` 与启动/关闭路径**不**迁移（前者跨多请求 batch 不存在单一 trace_id，后者无活跃 span）。详见 [`superpowers/plans/2026-04-28-broker-trace-propagation.md`](superpowers/plans/2026-04-28-broker-trace-propagation.md)。
```

- [ ] **Step 3: 在「已完成」段尾追加聚合条目**

定位 `## 已完成` 段下 `### [P2-1a] Ent driver ...` 条目之后，追加：

```markdown
### [P0-5] Kafka broker record ctx 丢失修复 ✅ 2026-04-28

抽出 `(s *kafkaSubscriber) dispatch` 方法，handler 收到 `r.Context ?? loopCtx`，让 kotel hook 已经填好的上游 span context 真正传到业务 handler。回归测试覆盖优先级 + nil 回落两种场景。详见 [`superpowers/plans/2026-04-28-broker-trace-propagation.md`](superpowers/plans/2026-04-28-broker-trace-propagation.md)。

### [P2-1b] Logger Helper Ctx 规范 ✅ 2026-04-28

`obs/logging/example_test.go` + `For/NewHelper` godoc 修订。业务侧 audit data 层 5 处真实关联缺口已迁移；架构上不该带 trace_id 的位置不动。详见 [`superpowers/plans/2026-04-28-broker-trace-propagation.md`](superpowers/plans/2026-04-28-broker-trace-propagation.md)。
```

- [ ] **Step 4: 移除 P2 主段中的 P2-1b 占位**

把 Step 2 中改写后位于 `## P2` 主段下的 P2-1b 整段移除（已迁到「已完成」段）。

- [ ] **Step 5: 校验文档自洽**

Run: `cd servora && grep -c "2026-04-28-broker-trace-propagation" docs/TODO.md`
Expected: ≥ 4。

Run: `cd servora && awk '/^## P2/,/^## P3/' docs/TODO.md | grep -c "P2-1b"`
Expected: `0`（P2 主段不再有 P2-1b 占位）。

- [ ] **Step 6: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
git add docs/TODO.md
git commit -m "docs(repo): close P2-1b, add P0-5 (broker record ctx fix)"
```

---

## Task 9: 全量回归 + 打 tag

**Files:** 无

- [ ] **Step 1: servora 仓库**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
go test -race ./infra/broker/... ./obs/logging/...
make ci.lint
```
Expected: 全部 PASS。

- [ ] **Step 2: servora-platform 仓库**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform
go test -race ./app/audit/service/internal/data/...
make lint
```
Expected: 全部 PASS。

- [ ] **Step 3: 打 tag**

本 plan 修改了 `infra/broker/kafka/`（行为变化，影响所有 broker 消费者）和 `obs/logging/`（godoc 修订）。属于必打 tag 的范畴。

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
make tag TAG=v0.x.y  # 替换为下一个 patch 版本号
```

- [ ] **Step 4: servora-platform 升级到新框架版本**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora-platform/app/audit/service
go get github.com/Servora-Kit/servora@v0.x.y
go mod tidy
git add go.mod go.sum
git commit -m "chore(audit): bump servora to v0.x.y for broker trace fix"
```

---

## Self-Review Checklist

- [x] **Spec 覆盖**：旧 plan 的 8 个 Task 中，真有用的 3 个（Example test / godoc / audit consumer 迁移）保留；错误的 3 个（AGENTS.md 整章 / batch_writer 全迁移 / worker_client 清理）剔除；缺失的 broker fix 提到核心位置（Task 1-3）
- [x] **No placeholder**：所有 step 含具体代码 / 命令 / expected 输出
- [x] **类型一致**：`dispatch(loopCtx, r)`、`broker.Handler`、`broker.Event`、`broker.SubscribeOptions{AutoAck bool}` 在所有 task 中签名一致（已对照 `infra/broker/message.go:48` 与 `infra/broker/options.go:29-31` 验证）
- [x] **TDD**：Task 2 写失败测试 → Task 3 修复 → 测试 PASS，标准 RED-GREEN
- [x] **跨仓库 cd**：每次 cd 路径明确
- [x] **依赖顺序**：Task 6 显式标注依赖 Task 3；Task 9 升级 servora-platform 依赖在 Task 8 tag 之后

## Risks & Mitigations

| 风险 | 缓解 |
|---|---|
| `r.Context` 在 kotel hook 未启用的部署中可能为 nil | Task 3 加 nil 回落 loopCtx，Task 2 单独测试覆盖 |
| `r.Context` 携带的 receive span 在 handler 返回后可能未 End（`OnFetchRecordUnbuffered` 才 End） | kotel 设计如此：consumer 处理期间 span 仍 active，handler 返回后 kgo 触发 unbuffered hook 自动 End。无需手工管理 |
| audit consumer.go L80（unsubscribe 失败）发生在 Stop 路径，签名 `Stop(_ context.Context)` 把 ctx 丢弃 | 不迁移此点（属于关闭路径，无活跃 span），与 plan 决策一致 |
| 升级 servora 版本可能破坏其它消费 broker.Subscribe 的业务（如 servora-iam） | 行为变化：handler 现在收到的 ctx 可能与之前不同（带 span vs 不带）。语义上更正确，业务代码不应依赖之前的「无 span」ctx；如有特殊场景在 release notes 提示 |

## Out of Scope

- `kotel.Tracer.WithProcessSpan(r)` 进一步起 "process" 子 span 并自动 End —— 进阶增强，本 plan 仅做最小修复（fallback ctx）。可作为 P3 follow-up
- 业务 lint 工具自动检测漏写 `WithContext(ctx)` —— 独立 P3
- `batch_writer.go` 跨请求 batch 的事件级 trace_id 标注（应作为日志字段而非 valuer，需重新设计）—— 独立小 RFC
- bootstrap 阶段日志的「invalid SpanContext 时省略字段」优化（P2-1d，独立处理）
- `servora-example` master/worker 实验日志清理 —— 独立小 PR

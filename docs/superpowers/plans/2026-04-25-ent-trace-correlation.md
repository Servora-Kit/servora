# Ent Trace Correlation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 servora 的 Ent SQL driver 加一层透明的 tracing wrapper，让每次 `Query`/`Exec`/`Tx` 在写结构化日志时自动带上当前 OTel trace_id 和 span_id，从而把 DB 调用与 HTTP/gRPC access log 关联起来。

**Architecture:** 新建一个实现 `dialect.Driver` 接口的 `tracingDriver`，转发所有方法到内层 driver，并在每次调用后从入参 `ctx` 抽 SpanContext，写一条 zap 结构化日志（含 sql、duration、trace_id、span_id、err）。同时实现 `dialect.Tx` 的 wrapper 让事务内的 `Query`/`Exec` 也带上 trace。Wrapper 通过新 constructor `NewDriverWithTracing` 显式启用，原 `NewDriver` 保持不变以零破坏。

**Tech Stack:**
- `entgo.io/ent/dialect` — `Driver` / `Tx` / `ExecQuerier` 接口
- `entgo.io/ent/dialect/sql` — 既有 `entsql.Driver`（被 wrap）
- `go.opentelemetry.io/otel/trace` — `SpanContextFromContext`
- `go.uber.org/zap` — 结构化日志（与 `obs/logging` 保持一致）
- `go.uber.org/zap/zaptest/observer` — 测试时捕获日志条目断言
- `github.com/mattn/go-sqlite3` — 集成测试使用 in-memory SQLite

---

## File Structure

| 文件 | 操作 | 责任 |
|---|---|---|
| `servora/infra/db/ent/tracing.go` | Create | `tracingDriver` + `tracingTx` + `WrapWithTracing` 工厂 |
| `servora/infra/db/ent/tracing_test.go` | Create | 单元测试（fake driver + 注入 span） |
| `servora/infra/db/ent/tracing_integration_test.go` | Create | 集成测试（sqlite3 in-memory + 真实 ent client） |
| `servora/infra/db/ent/driver.go` | Modify | 新增 `NewDriverWithTracing` 便利 constructor |
| `servora/obs/logging/ent_log.go` | Modify | 在 `EntLogFuncFrom` 上加 `Deprecated:` godoc，引导用 driver wrapper |
| `servora/AGENTS.md` | Modify | 在「常用命令」之前加 "Ent Tracing 集成" 小节，举例正确用法 |

---

## Task 1: 建立 tracing.go 骨架与 fake driver 测试桩

**Files:**
- Create: `servora/infra/db/ent/tracing.go`
- Create: `servora/infra/db/ent/tracing_test.go`

- [ ] **Step 1: 写第一个失败测试 — fake driver 收到 Query 调用次数**

```go
// servora/infra/db/ent/tracing_test.go
package ent_test

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"

	servoraent "github.com/Servora-Kit/servora/infra/db/ent"
)

type fakeDriver struct {
	dialect.Driver
	queryCalls int
	execCalls  int
}

func (f *fakeDriver) Query(ctx context.Context, query string, args, v any) error {
	f.queryCalls++
	return nil
}
func (f *fakeDriver) Exec(ctx context.Context, query string, args, v any) error {
	f.execCalls++
	return nil
}
func (f *fakeDriver) Close() error    { return nil }
func (f *fakeDriver) Dialect() string { return "fake" }

func TestWrapWithTracing_DelegatesQuery(t *testing.T) {
	inner := &fakeDriver{}
	wrapped := servoraent.WrapWithTracing(inner, zap.NewNop())

	if err := wrapped.Query(context.Background(), "SELECT 1", nil, nil); err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if inner.queryCalls != 1 {
		t.Errorf("expected 1 Query call on inner, got %d", inner.queryCalls)
	}
}
```

- [ ] **Step 2: 跑测试确认编译失败**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run TestWrapWithTracing_DelegatesQuery`
Expected: 编译失败，`undefined: ent.WrapWithTracing`

- [ ] **Step 3: 在 tracing.go 中实现最小框架让测试通过**

```go
// servora/infra/db/ent/tracing.go
package ent

import (
	"context"

	"entgo.io/ent/dialect"
	"go.uber.org/zap"
)

// tracingDriver wraps a dialect.Driver and emits zap logs with OTel trace
// context for every Query/Exec/Tx call.
type tracingDriver struct {
	inner dialect.Driver
	log   *zap.Logger
}

// WrapWithTracing returns a dialect.Driver that delegates to inner and emits
// trace-correlated logs through log. Pass zap.NewNop() to disable logging.
func WrapWithTracing(inner dialect.Driver, log *zap.Logger) dialect.Driver {
	if log == nil {
		log = zap.NewNop()
	}
	return &tracingDriver{inner: inner, log: log}
}

func (d *tracingDriver) Query(ctx context.Context, query string, args, v any) error {
	return d.inner.Query(ctx, query, args, v)
}

func (d *tracingDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.inner.Exec(ctx, query, args, v)
}

func (d *tracingDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	return d.inner.Tx(ctx)
}

func (d *tracingDriver) Close() error    { return d.inner.Close() }
func (d *tracingDriver) Dialect() string { return d.inner.Dialect() }
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run TestWrapWithTracing_DelegatesQuery -v`
Expected: `--- PASS: TestWrapWithTracing_DelegatesQuery`

- [ ] **Step 5: 提交**

```bash
cd /Users/horonlee/projects/go/servora-kit/servora
git add infra/db/ent/tracing.go infra/db/ent/tracing_test.go
git commit -m "feat(infra/db/ent): add tracing driver wrapper skeleton"
```

---

## Task 2: 实现 Query 路径的 trace_id/span_id 日志

**Files:**
- Modify: `servora/infra/db/ent/tracing.go`
- Modify: `servora/infra/db/ent/tracing_test.go`

- [ ] **Step 1: 写失败测试 — Query 失败时日志含 trace_id/span_id 和 err**

在 `tracing_test.go` import 块追加：

```go
"errors"

"go.opentelemetry.io/otel/trace"
"go.uber.org/zap/zapcore"
"go.uber.org/zap/zaptest/observer"
```

文件末尾追加：

```go
type erroringDriver struct{ fakeDriver }

func (e *erroringDriver) Query(ctx context.Context, q string, a, v any) error {
	return errors.New("boom")
}

func TestWrapWithTracing_QueryErrorLogsTraceID(t *testing.T) {
	core, recorded := observer.New(zapcore.DebugLevel)
	zlog := zap.New(core)

	wrapped := servoraent.WrapWithTracing(&erroringDriver{}, zlog)

	traceID, _ := trace.TraceIDFromHex("0c655cd077ab20bd780fa1504e4da2c9")
	spanID, _ := trace.SpanIDFromHex("c16e5b0589450eb0")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	err := wrapped.Query(ctx, "SELECT 1", nil, nil)
	if err == nil {
		t.Fatal("expected error from inner driver")
	}

	logs := recorded.All()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	got := logs[0].ContextMap()
	if got["trace_id"] != "0c655cd077ab20bd780fa1504e4da2c9" {
		t.Errorf("trace_id mismatch: %v", got["trace_id"])
	}
	if got["span_id"] != "c16e5b0589450eb0" {
		t.Errorf("span_id mismatch: %v", got["span_id"])
	}
	if got["sql"] != "SELECT 1" {
		t.Errorf("sql mismatch: %v", got["sql"])
	}
	if got["error"] != "boom" {
		t.Errorf("error field mismatch: %v", got["error"])
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run TestWrapWithTracing_QueryErrorLogsTraceID -v`
Expected: FAIL，`expected 1 log entry, got 0`

- [ ] **Step 3: 实现 Query 日志逻辑**

在 `tracing.go` import 块加入：

```go
"time"

"go.opentelemetry.io/otel/trace"
"go.uber.org/zap/zapcore"
```

替换 `Query` 方法并新增 helper：

```go
func (d *tracingDriver) Query(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := d.inner.Query(ctx, query, args, v)
	d.logCall(ctx, "ent.query", query, time.Since(start), err)
	return err
}

func (d *tracingDriver) logCall(ctx context.Context, op, sql string, elapsed time.Duration, err error) {
	logCallTo(d.log, ctx, op, sql, elapsed, err)
}

// logCallTo emits a single structured log line with trace correlation.
// Shared by both tracingDriver and tracingTx.
func logCallTo(zlog *zap.Logger, ctx context.Context, op, sql string, elapsed time.Duration, err error) {
	fields := make([]zapcore.Field, 0, 6)
	if sql != "" {
		fields = append(fields, zap.String("sql", sql))
	}
	if elapsed > 0 {
		fields = append(fields, zap.Duration("elapsed", elapsed))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields,
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		zlog.Error(op, fields...)
		return
	}
	zlog.Debug(op, fields...)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run TestWrapWithTracing_QueryErrorLogsTraceID -v`
Expected: PASS

- [ ] **Step 5: 跑全部包内测试**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -v`
Expected: All PASS

- [ ] **Step 6: 提交**

```bash
git add infra/db/ent/tracing.go infra/db/ent/tracing_test.go
git commit -m "feat(infra/db/ent): emit trace-correlated logs on Query"
```

---

## Task 3: Query 成功路径 + Exec 路径

**Files:**
- Modify: `servora/infra/db/ent/tracing.go`
- Modify: `servora/infra/db/ent/tracing_test.go`

- [ ] **Step 1: 写失败测试 — Query 成功是 Debug 级，Exec 也写日志**

在 `tracing_test.go` 末尾追加：

```go
func TestWrapWithTracing_QuerySuccessDebugLog(t *testing.T) {
	core, recorded := observer.New(zapcore.DebugLevel)
	zlog := zap.New(core)

	wrapped := servoraent.WrapWithTracing(&fakeDriver{}, zlog)
	if err := wrapped.Query(context.Background(), "SELECT 1", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := recorded.All()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Level != zapcore.DebugLevel {
		t.Errorf("expected Debug level, got %v", logs[0].Level)
	}
	if logs[0].Message != "ent.query" {
		t.Errorf("expected message ent.query, got %q", logs[0].Message)
	}
}

func TestWrapWithTracing_ExecMirrorsQuery(t *testing.T) {
	core, recorded := observer.New(zapcore.DebugLevel)
	wrapped := servoraent.WrapWithTracing(&fakeDriver{}, zap.New(core))

	if err := wrapped.Exec(context.Background(), "INSERT INTO t VALUES(?)", []any{1}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logs := recorded.All()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Message != "ent.exec" {
		t.Errorf("expected message ent.exec, got %q", logs[0].Message)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run "TestWrapWithTracing_QuerySuccessDebugLog|TestWrapWithTracing_ExecMirrorsQuery" -v`
Expected: 第一个 PASS，第二个 FAIL（`expected message ent.exec, got ""`）

- [ ] **Step 3: 实现 Exec 日志**

替换 `tracing.go` 中的 `Exec` 方法：

```go
func (d *tracingDriver) Exec(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := d.inner.Exec(ctx, query, args, v)
	d.logCall(ctx, "ent.exec", query, time.Since(start), err)
	return err
}
```

- [ ] **Step 4: 跑全部包内测试**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -v`
Expected: All PASS

- [ ] **Step 5: 提交**

```bash
git add infra/db/ent/tracing.go infra/db/ent/tracing_test.go
git commit -m "feat(infra/db/ent): instrument Exec with trace-correlated logs"
```

---

## Task 4: 包装事务 (Tx 子接口)

**Files:**
- Modify: `servora/infra/db/ent/tracing.go`
- Modify: `servora/infra/db/ent/tracing_test.go`

- [ ] **Step 1: 写失败测试 — Tx.Query 也带 trace_id, Commit 写一条 ent.tx.commit**

在 `tracing_test.go` 末尾追加：

```go
type fakeTx struct {
	queryCalls int
	committed  bool
	rolled     bool
}

func (t *fakeTx) Query(ctx context.Context, q string, a, v any) error { t.queryCalls++; return nil }
func (t *fakeTx) Exec(ctx context.Context, q string, a, v any) error  { return nil }
func (t *fakeTx) Commit() error                                       { t.committed = true; return nil }
func (t *fakeTx) Rollback() error                                     { t.rolled = true; return nil }

type txProvidingDriver struct {
	fakeDriver
	tx *fakeTx
}

func (d *txProvidingDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	d.tx = &fakeTx{}
	return d.tx, nil
}

func TestWrapWithTracing_TxQueryEmitsTraceLog(t *testing.T) {
	core, recorded := observer.New(zapcore.DebugLevel)
	inner := &txProvidingDriver{}
	wrapped := servoraent.WrapWithTracing(inner, zap.New(core))

	traceID, _ := trace.TraceIDFromHex("aa93007433f6fed9d671232f325c08e8")
	spanID, _ := trace.SpanIDFromHex("056b6dd0acd286d2")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	tx, err := wrapped.Tx(ctx)
	if err != nil {
		t.Fatalf("Tx error: %v", err)
	}
	if err := tx.Query(ctx, "SELECT 2", nil, nil); err != nil {
		t.Fatalf("tx.Query error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("tx.Commit error: %v", err)
	}

	logs := recorded.All()
	// expect: ent.tx.begin + ent.tx.query + ent.tx.commit
	if len(logs) != 3 {
		t.Fatalf("expected 3 log entries, got %d: %+v", len(logs), logs)
	}
	queryEntry := logs[1]
	got := queryEntry.ContextMap()
	if got["trace_id"] != "aa93007433f6fed9d671232f325c08e8" {
		t.Errorf("trace_id missing or mismatch in tx.query: %v", got["trace_id"])
	}
	if inner.tx.queryCalls != 1 || !inner.tx.committed {
		t.Errorf("inner tx not exercised correctly: %+v", inner.tx)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run TestWrapWithTracing_TxQueryEmitsTraceLog -v`
Expected: FAIL，`expected 3 log entries, got 0`

- [ ] **Step 3: 实现 tracingTx 并替换 (*tracingDriver).Tx**

在 `tracing.go` 末尾追加：

```go
// tracingTx wraps a dialect.Tx and forwards trace context to per-call logs.
// ctx captured at Begin time is reused for Commit/Rollback log correlation.
type tracingTx struct {
	inner dialect.Tx
	log   *zap.Logger
	ctx   context.Context
}

func (t *tracingTx) Query(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := t.inner.Query(ctx, query, args, v)
	logCallTo(t.log, ctx, "ent.tx.query", query, time.Since(start), err)
	return err
}

func (t *tracingTx) Exec(ctx context.Context, query string, args, v any) error {
	start := time.Now()
	err := t.inner.Exec(ctx, query, args, v)
	logCallTo(t.log, ctx, "ent.tx.exec", query, time.Since(start), err)
	return err
}

func (t *tracingTx) Commit() error {
	err := t.inner.Commit()
	logCallTo(t.log, t.ctx, "ent.tx.commit", "", 0, err)
	return err
}

func (t *tracingTx) Rollback() error {
	err := t.inner.Rollback()
	logCallTo(t.log, t.ctx, "ent.tx.rollback", "", 0, err)
	return err
}
```

替换 `(*tracingDriver).Tx`：

```go
func (d *tracingDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	tx, err := d.inner.Tx(ctx)
	logCallTo(d.log, ctx, "ent.tx.begin", "", 0, err)
	if err != nil {
		return nil, err
	}
	return &tracingTx{inner: tx, log: d.log, ctx: ctx}, nil
}
```

- [ ] **Step 4: 跑全部测试确认通过**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -v`
Expected: All PASS

- [ ] **Step 5: 提交**

```bash
git add infra/db/ent/tracing.go infra/db/ent/tracing_test.go
git commit -m "feat(infra/db/ent): wrap Tx to propagate trace context"
```

---

## Task 5: 集成测试 (sqlite3 in-memory)

**Files:**
- Create: `servora/infra/db/ent/tracing_integration_test.go`

- [ ] **Step 1: 写集成测试 — 真实 dialect 验证日志产出**

```go
//go:build integration

package ent_test

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	servoraent "github.com/Servora-Kit/servora/infra/db/ent"
)

func TestIntegration_SqliteExecWithTrace(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	core, recorded := observer.New(zapcore.DebugLevel)
	var inner dialect.Driver = entsql.OpenDB(dialect.SQLite, db)
	wrapped := servoraent.WrapWithTracing(inner, zap.New(core))

	traceID, _ := trace.TraceIDFromHex("11111111111111111111111111111111")
	spanID, _ := trace.SpanIDFromHex("2222222222222222")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	if err := wrapped.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)", nil, nil); err != nil {
		t.Fatalf("Exec CREATE: %v", err)
	}
	if err := wrapped.Exec(ctx, "INSERT INTO users(name) VALUES(?)", []any{"alice"}, nil); err != nil {
		t.Fatalf("Exec INSERT: %v", err)
	}

	logs := recorded.All()
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(logs))
	}
	got := logs[0].ContextMap()
	if got["trace_id"] != "11111111111111111111111111111111" {
		t.Errorf("trace_id mismatch on first log: %v", got["trace_id"])
	}
	if got["span_id"] != "2222222222222222" {
		t.Errorf("span_id mismatch on first log: %v", got["span_id"])
	}
}
```

- [ ] **Step 2: 跑集成测试**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test -tags=integration ./infra/db/ent/ -run TestIntegration_SqliteExecWithTrace -v`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add infra/db/ent/tracing_integration_test.go
git commit -m "test(infra/db/ent): integration test for tracing wrapper with sqlite3"
```

---

## Task 6: 暴露 NewDriverWithTracing 工厂

**Files:**
- Modify: `servora/infra/db/ent/driver.go`
- Modify: `servora/infra/db/ent/tracing_test.go`

- [ ] **Step 1: 写失败测试 — NewDriverWithTracing 返回的 driver 自带日志**

在 `tracing_test.go` import 块追加：

```go
conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
```

文件末尾追加：

```go
func TestNewDriverWithTracing_WrapsTransparently(t *testing.T) {
	cfg := &conf.Data{
		Database: &conf.Data_Database{Driver: "sqlite", Source: ":memory:"},
	}
	core, recorded := observer.New(zapcore.DebugLevel)

	drv, err := servoraent.NewDriverWithTracing(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("NewDriverWithTracing: %v", err)
	}
	defer drv.Close()

	if err := drv.Exec(context.Background(), "CREATE TABLE t (id INT)", nil, nil); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got := len(recorded.All()); got == 0 {
		t.Errorf("expected log entry from wrapped driver, got 0")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -run TestNewDriverWithTracing_WrapsTransparently`
Expected: FAIL，`undefined: ent.NewDriverWithTracing`

- [ ] **Step 3: 在 driver.go 增加工厂函数**

在 `driver.go` import 块追加：

```go
"entgo.io/ent/dialect"
"go.uber.org/zap"
```

在文件末尾追加：

```go
// NewDriverWithTracing creates a driver via NewDriver and wraps it with the
// tracing decorator. Pass the zap logger from your obs/logging.ZapLogger:
//
//	drv, err := ent.NewDriverWithTracing(cfg, zapLogger.Zap())
func NewDriverWithTracing(cfg *conf.Data, log *zap.Logger) (dialect.Driver, error) {
	inner, err := NewDriver(cfg)
	if err != nil {
		return nil, err
	}
	return WrapWithTracing(inner, log), nil
}
```

- [ ] **Step 4: 跑全部测试确认通过**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test ./infra/db/ent/ -v`
Expected: All PASS

- [ ] **Step 5: 提交**

```bash
git add infra/db/ent/driver.go infra/db/ent/tracing_test.go
git commit -m "feat(infra/db/ent): add NewDriverWithTracing convenience constructor"
```

---

## Task 7: 在旧 ent_log.go 加 Deprecated 注释

**Files:**
- Modify: `servora/obs/logging/ent_log.go`

- [ ] **Step 1: 修改 godoc 引导用 driver wrapper**

替换 `EntLogFuncFrom` 函数上方的注释（替换 `func EntLogFuncFrom` 之前的所有注释行）：

```go
// EntLogFuncFrom returns a logger function compatible with ent.Log() option.
//
// Deprecated: This adapter cannot propagate OpenTelemetry trace context because
// ent.Log()'s signature is `func(...any)` with no context parameter. For
// trace-correlated SQL logs, use ent.NewDriverWithTracing in
// `servora/infra/db/ent` instead — it wraps the dialect.Driver and emits
// structured logs with trace_id/span_id extracted from each call's ctx.
func EntLogFuncFrom(logger kratoslog.Logger, module string) func(...any) {
```

- [ ] **Step 2: 验证 go build 不破坏**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go build ./...`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add obs/logging/ent_log.go
git commit -m "docs(obs/logging): deprecate EntLogFuncFrom in favor of driver wrapper"
```

---

## Task 8: 更新 servora/AGENTS.md 文档

**Files:**
- Modify: `servora/AGENTS.md`

- [ ] **Step 1: 在「常用命令」前插入 "Ent Tracing 集成" 小节**

定位到 `## 常用命令` 行（即 ` ```bash` 代码块开始处的标题），**在其上方** 插入：

````markdown
## Ent Tracing 集成

业务仓库使用 Ent 时，**必须** 通过 `infra/db/ent.NewDriverWithTracing` 创建 driver，让所有 SQL 调用日志自动带上当前 OTel trace_id/span_id：

```go
import (
    servoraent "github.com/Servora-Kit/servora/infra/db/ent"
    "github.com/Servora-Kit/servora/obs/logging"
)

func NewEntClient(cfg *conf.Data, l logging.Logger) (*ent.Client, error) {
    zapLogger, _ := l.(*logging.ZapLogger)
    drv, err := servoraent.NewDriverWithTracing(cfg, zapLogger.Zap())
    if err != nil {
        return nil, err
    }
    return ent.NewClient(ent.Driver(drv)), nil
}
```

每条 SQL 调用会输出 `ent.query` / `ent.exec` / `ent.tx.{begin,query,exec,commit,rollback}` 日志，含 `trace_id` / `span_id` / `sql` / `elapsed` / `error` 字段。失败为 ERROR 级，成功为 DEBUG 级。

> 旧的 `obs/logging.EntLogFuncFrom` 已弃用——其接口签名不带 ctx，无法做 trace 关联。
````

- [ ] **Step 2: 验证 markdown 中字符串确实写入**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && grep -c "Ent Tracing 集成" AGENTS.md`
Expected: `1`

- [ ] **Step 3: 提交**

```bash
git add AGENTS.md
git commit -m "docs(repo): document Ent tracing wrapper usage"
```

---

## Task 9: 全量回归 + 关闭 TODO

**Files:**
- Modify: `servora/docs/TODO.md`

- [ ] **Step 1: 跑所有相关测试 (含 race)**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test -race ./infra/db/ent/... ./obs/logging/...`
Expected: All PASS

- [ ] **Step 2: 跑集成测试**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && go test -tags=integration -race ./infra/db/ent/...`
Expected: All PASS

- [ ] **Step 3: 跑 lint**

Run: `cd /Users/horonlee/projects/go/servora-kit/servora && make lint`
Expected: 无新增 lint 错误

- [ ] **Step 4: 把 P2-1a 条目从 P2 段移到「已完成」段**

定位 `servora/docs/TODO.md` 的 `## 已完成` 段（当前内容是「（暂无）」），替换为：

```markdown
## 已完成

### [P2-1a] Ent driver 加 trace_id/span_id ✅ 2026-MM-DD

通过 `infra/db/ent.NewDriverWithTracing` 包装 dialect.Driver，每次 Query/Exec/Tx 自动写出含 trace_id/span_id 的结构化日志。详见 `docs/superpowers/plans/2026-04-25-ent-trace-correlation.md`。
```

并删除原 P2 段中的 P2-1a 条目（如果 P2-1 已被拆分为 a/b/c/d，删除对应 a 那条；如还是合并为一条 P2-1，则按已确定的拆分先把 b/c/d 留下）。

- [ ] **Step 5: 提交并打 tag**

```bash
git add docs/TODO.md
git commit -m "docs(repo): close P2-1a — Ent trace correlation shipped"
make tag TAG=v0.x.y  # 替换为下一个版本号
```

---

## Self-Review Checklist

- [x] 每个 task 都有「失败测试 → 实现 → 通过 → commit」完整流程
- [x] 所有代码 block 含完整 import 与函数签名
- [x] 命令行均含 `cd /Users/horonlee/projects/go/servora-kit/servora` 前缀
- [x] 测试断言都是具体值（trace_id 字面量），无 placeholder
- [x] Task 之间符号一致：`WrapWithTracing` / `tracingDriver` / `tracingTx` / `logCallTo` / `NewDriverWithTracing` 全文统一
- [x] AGENTS.md 修改前后位置明确（在「常用命令」上方）
- [x] 完工动作（关 TODO + tag）有具体 step

## Risks & Mitigations

| 风险 | 缓解 |
|---|---|
| `entsql.OpenDB` 返回的 `*entsql.Driver` 实际类型是否在 ent 内部被 `type assert` 用过 | Task 5 集成测试用 sqlite 真跑 Exec 验证；如发现问题改为持有 `*entsql.Driver` 而非接口 |
| Tx 方法忘记包装会导致事务内日志失联 | Task 4 专门覆盖该路径，含 commit 断言 |
| 日志记录 args 可能泄漏敏感数据（如密码 hash） | 当前实现**不**记录 args，仅记 sql 文本；未来需要时另起 redaction option |
| Debug 级日志在 prod 默认被过滤 | 这是预期行为；失败路径用 Error 级保证可见 |
| `*conf.Data_Database` 字段名 (`Driver`/`Source`) 与生成代码不一致 | Task 6 实施前先 `grep -n "Database" /servora/api/gen/go/servora/conf/v1/*.pb.go` 核对字段名 |

## Out of Scope

- GORM 慢查询日志的 trace 关联（属 P2-1c，单独 plan）
- `obs/logging.EntLogFuncFrom` 的实际删除（先 Deprecated 一两个版本再删）
- DB 慢查询阈值告警（业务侧自行配置 SlowThreshold，与本 plan 无关）
- 业务侧 `r.log.Warnf` 的 ctx 绑定（属 P2-1b，单独 plan）

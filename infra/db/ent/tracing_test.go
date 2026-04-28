package ent_test

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
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

func TestNewDriverWithTracing_WrapsTransparently(t *testing.T) {
	cfg := &conf.Data{
		Database: &conf.Data_Database{Driver: "sqlite", Source: ":memory:"},
	}
	core, recorded := observer.New(zapcore.DebugLevel)

	drv, err := servoraent.NewDriverWithTracing(cfg, zap.New(core))
	if err != nil {
		t.Fatalf("NewDriverWithTracing: %v", err)
	}
	defer func() { _ = drv.Close() }()

	if err := drv.Exec(context.Background(), "CREATE TABLE t (id INT)", []any{}, nil); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got := len(recorded.All()); got == 0 {
		t.Errorf("expected log entry from wrapped driver, got 0")
	}
}

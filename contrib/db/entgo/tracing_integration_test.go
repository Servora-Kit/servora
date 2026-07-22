//go:build integration

package ent_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	servoraent "github.com/Servora-Kit/servora/contrib/db/entgo"
)

func TestIntegration_SqliteExecWithTrace(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	core, recorded := observer.New(zapcore.DebugLevel)
	wrapped, err := servoraent.NewDriver(
		&corev1.Data{Database: &corev1.Data_Database{Driver: "sqlite"}},
		servoraent.WithDB(db),
		servoraent.WithTracing(zap.New(core)),
	)
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}

	traceID, _ := trace.TraceIDFromHex("11111111111111111111111111111111")
	spanID, _ := trace.SpanIDFromHex("2222222222222222")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	if err := wrapped.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)", []any{}, nil); err != nil {
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

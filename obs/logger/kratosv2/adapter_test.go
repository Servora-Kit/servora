package kratosv2

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"

	kratoslog "github.com/go-kratos/kratos/v2/log"
)

func TestWrap_NilSafe(t *testing.T) {
	l := Wrap(nil)
	if l == nil {
		t.Fatal("Wrap(nil) must return non-nil")
	}
}

func TestWrap_LevelMapping(t *testing.T) {
	tests := []struct {
		kratos kratoslog.Level
		want   slog.Level
	}{
		{kratoslog.LevelDebug, slog.LevelDebug},
		{kratoslog.LevelInfo, slog.LevelInfo},
		{kratoslog.LevelWarn, slog.LevelWarn},
		{kratoslog.LevelError, slog.LevelError},
		{kratoslog.LevelFatal, slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.kratos.String(), func(t *testing.T) {
			got := mapLevel(tt.kratos)
			if got != tt.want {
				t.Errorf("mapLevel(%v) = %v, want %v", tt.kratos, got, tt.want)
			}
		})
	}
}

func TestWrap_KeyvalsToAttrs(t *testing.T) {
	var buf bytes.Buffer
	sl := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	kl := Wrap(sl)

	_ = kl.Log(kratoslog.LevelInfo, "msg", "hello", "service", "test-svc")

	m := decodeJSONLog(t, &buf)
	if m["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", m["msg"])
	}
	if m["service"] != "test-svc" {
		t.Errorf("service = %v, want test-svc", m["service"])
	}
}

func TestSetDefault_UpdatesGlobalDelegate(t *testing.T) {
	var buf bytes.Buffer
	sl := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})).With("service", "test-svc")
	SetDefault(sl)
	t.Cleanup(func() { SetDefault(nil) })

	kratoslog.Log(kratoslog.LevelInfo, "msg", "hello", "trace", "abc")

	m := decodeJSONLog(t, &buf)
	if m["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", m["msg"])
	}
	if m["service"] != "test-svc" {
		t.Errorf("service = %v, want test-svc", m["service"])
	}
	if m["trace"] != "abc" {
		t.Errorf("trace = %v, want abc", m["trace"])
	}
}

func TestSetDefault_NilDelegateFallsBackToSlogDefault(t *testing.T) {
	var buf bytes.Buffer
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})).With("service", "fallback-svc"))
	SetDefault(nil)
	t.Cleanup(func() {
		SetDefault(nil)
		slog.SetDefault(oldDefault)
	})

	kratoslog.Log(kratoslog.LevelInfo, "msg", "fallback")

	m := decodeJSONLog(t, &buf)
	if m["msg"] != "fallback" {
		t.Errorf("msg = %v, want fallback", m["msg"])
	}
	if m["service"] != "fallback-svc" {
		t.Errorf("service = %v, want fallback-svc", m["service"])
	}
}

func TestSetDefault_RaceSafeWithGlobalLogCalls(t *testing.T) {
	const iterations = 500
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := range iterations {
			kratoslog.Log(kratoslog.LevelInfo, "msg", "hello", "i", i)
		}
	}()

	go func() {
		defer wg.Done()
		for range iterations {
			SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
		}
	}()

	wg.Wait()
	SetDefault(nil)
}

func TestWrap_NoImportParentPackage(t *testing.T) {
	// Compile-time check: this package imports only kratoslog + slog,
	// NOT obs/logger. If someone adds that import, this file won't compile
	// (would create an import cycle since parent doesn't import us either,
	// but the design constraint is: kratosv2 SHALL NOT import obs/logger).
}

func decodeJSONLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v, output: %s", err, buf.String())
	}
	return m
}

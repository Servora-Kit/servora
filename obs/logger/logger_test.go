package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestNew_NilSafe(t *testing.T) {
	l, closer := New(nil)
	if l == nil {
		t.Fatal("New(nil) must return non-nil *slog.Logger")
	}
	if closer == nil {
		t.Fatal("closer must be non-nil")
	}
	if err := closer(context.Background()); err != nil {
		t.Fatalf("closer error: %v", err)
	}
}

func TestNew_DefaultStdoutWhenEmpty(t *testing.T) {
	l, _ := New(&corev1.Bootstrap{
		App: &corev1.App{Env: "dev"},
		Obs: &corev1.Observability{Log: &corev1.Log{}},
	})
	if l == nil {
		t.Fatal("empty backends must default to stdout logger")
	}
}

func TestNew_NoopBackend(t *testing.T) {
	l, _ := New(&corev1.Bootstrap{
		Obs: &corev1.Observability{Log: &corev1.Log{
			Backends: []*corev1.Log_LogBackend{{
				Backend: &corev1.Log_LogBackend_Noop{Noop: &corev1.Log_NoopBackend{}},
			}},
		}},
	})
	l.Info("should not panic")
}

func TestResolveStdoutJSON_EnvDefaults(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"dev=text", "dev", false},
		{"empty=text", "", false},
		{"prod=json", "prod", true},
		{"staging=json", "staging", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveStdoutJSON(nil, tt.env)
			if got != tt.want {
				t.Errorf("resolveStdoutJSON(nil, %q) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestResolveStdoutJSON_ProtoOverridesEnv(t *testing.T) {
	textCfg := &corev1.Log_StdoutBackend{Format: corev1.Log_LOG_FORMAT_TEXT}
	if resolveStdoutJSON(textCfg, "prod") {
		t.Error("explicit TEXT must override prod json default")
	}

	jsonCfg := &corev1.Log_StdoutBackend{Format: corev1.Log_LOG_FORMAT_JSON}
	if !resolveStdoutJSON(jsonCfg, "dev") {
		t.Error("explicit JSON must override dev text default")
	}
}

func TestWithLogHandlerFunc_ReplacesStdout(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(
		&corev1.Bootstrap{
			App: &corev1.App{Env: "prod"},
			Obs: &corev1.Observability{Log: &corev1.Log{
				Backends: []*corev1.Log_LogBackend{{
					Backend: &corev1.Log_LogBackend_Stdout{
						Stdout: &corev1.Log_StdoutBackend{},
					},
				}},
			}},
		},
		WithLogHandlerFunc(func(_ io.Writer, lvl slog.Level) slog.Handler {
			return slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: lvl})
		}),
	)
	l.Info("hello", "key", "val")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("custom factory output expected to contain 'hello', got: %s", buf.String())
	}
}

func TestLevelOf(t *testing.T) {
	tests := []struct {
		proto corev1.Log_LogLevel
		want  slog.Level
	}{
		{corev1.Log_LOG_LEVEL_UNSPECIFIED, slog.LevelInfo},
		{corev1.Log_LOG_LEVEL_DEBUG, slog.LevelDebug},
		{corev1.Log_LOG_LEVEL_INFO, slog.LevelInfo},
		{corev1.Log_LOG_LEVEL_WARN, slog.LevelWarn},
		{corev1.Log_LOG_LEVEL_ERROR, slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.proto.String(), func(t *testing.T) {
			got := levelOf(&corev1.Log{Level: tt.proto})
			if got != tt.want {
				t.Errorf("levelOf(%v) = %v, want %v", tt.proto, got, tt.want)
			}
		})
	}
}

func TestIsTerminal_Injectable(t *testing.T) {
	orig := isTerminal
	t.Cleanup(func() { isTerminal = orig })

	isTerminal = func(uintptr) bool { return true }
	if !isTerminal(0) {
		t.Error("mock should return true")
	}

	isTerminal = func(uintptr) bool { return false }
	if isTerminal(0) {
		t.Error("mock should return false")
	}
}

func TestFanout_DispatchesToAll(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewJSONHandler(&buf1, nil)
	h2 := slog.NewJSONHandler(&buf2, nil)
	l := slog.New(fanout([]slog.Handler{h1, h2}))

	l.Info("fanout-test")

	if !strings.Contains(buf1.String(), "fanout-test") {
		t.Error("handler 1 did not receive record")
	}
	if !strings.Contains(buf2.String(), "fanout-test") {
		t.Error("handler 2 did not receive record")
	}
}

func TestFanout_SingleHandler(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, nil)
	got := fanout([]slog.Handler{h})
	if got != h {
		t.Error("single handler fanout should return handler directly")
	}
}

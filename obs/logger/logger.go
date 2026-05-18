package logger

import (
	"context"
	"log/slog"
	"sync/atomic"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	slogzap "github.com/samber/slog-zap/v2"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var current atomic.Pointer[zap.Logger]

// traceAttrs 从 ctx 的 OTEL SpanContext 提取 trace/span attr，供 slog-zap
// 的 AttrFromContext 扩展点逐条日志调用。无有效 span 时返回 nil（不注入
// 空值、不 panic），调用点无需感知。
func traceAttrs(ctx context.Context) []slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String(TraceIDKey, sc.TraceID().String()),
		slog.String(SpanIDKey, sc.SpanID().String()),
	}
}

// NewHandler 装配默认 handler：slog-zap(backend，复用 servora zap 定制)，
// trace/span 经其内建 AttrFromContext 扩展点由 traceAttrs 逐条日志注入。
func NewHandler(app *corev1.App) slog.Handler {
	z := buildZap(app)
	current.Store(z)
	return slogzap.Option{
		Logger:          z,
		AttrFromContext: []func(context.Context) []slog.Attr{traceAttrs},
	}.NewZapHandler()
}

// NewLogger = slog.New(NewHandler(app))，Layer 1 便利门面。
func NewLogger(app *corev1.App) *slog.Logger { return slog.New(NewHandler(app)) }

// Sync flush 底层 zap buffer，进程退出时由 Runtime cleanup 调用。
func Sync() error {
	if z := current.Load(); z != nil {
		return z.Sync()
	}
	return nil
}

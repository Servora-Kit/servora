package logger

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceIDKey / SpanIDKey 是 trace 关联的 slog attr key。与 ScopeKey 同样
// 单点定义，便于将来走 OTEL log bridge 时把它们映射到 LogRecord 的
// TraceId / SpanId 字段时收口于此。
const (
	TraceIDKey = "trace_id"
	SpanIDKey  = "span_id"
)

type contextHandler struct{ slog.Handler }

func newContextHandler(inner slog.Handler) slog.Handler { return &contextHandler{inner} }

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String(TraceIDKey, sc.TraceID().String()),
			slog.String(SpanIDKey, sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &contextHandler{h.Handler.WithAttrs(a)}
}

func (h *contextHandler) WithGroup(n string) slog.Handler {
	return &contextHandler{h.Handler.WithGroup(n)}
}

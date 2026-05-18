package logger

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type contextHandler struct{ slog.Handler }

func newContextHandler(inner slog.Handler) slog.Handler { return &contextHandler{inner} }

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &contextHandler{h.Handler.WithAttrs(a)}
}

func (h *contextHandler) WithGroup(n string) slog.Handler {
	return &contextHandler{h.Handler.WithGroup(n)}
}

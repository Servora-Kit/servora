package logger

// TraceIDKey / SpanIDKey 是 trace 关联的 slog attr key。与 ScopeKey 同样
// 单点定义，便于将来走 OTEL log bridge 时把它们映射到 LogRecord 的
// TraceId / SpanId 字段时收口于此。
const (
	TraceIDKey = "trace_id"
	SpanIDKey  = "span_id"
)

package logger

import "log/slog"

// ScopeKey 是组件标识的 slog attr key，对齐 OTEL InstrumentationScope.Name。
// 走 OTEL log bridge 时此 key 映射到 InstrumentationScope.Name 的逻辑集中于此。
const ScopeKey = "scope"

// Scope 返回绑定组件标识的新 logger，包级初始化时调用一次。
func Scope(l *slog.Logger, name string) *slog.Logger { return l.With(ScopeKey, name) }

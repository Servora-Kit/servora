package kratosv2

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	kratoslog "github.com/go-kratos/kratos/v2/log"
)

var defaultLogger = &atomicLogger{}

func init() {
	kratoslog.SetLogger(defaultLogger)
}

type slogAdapter struct {
	logger *slog.Logger
}

type atomicLogger struct {
	logger atomic.Pointer[slog.Logger]
}

// Wrap adapts a *slog.Logger to kratos v2 log.Logger.
// Self-contained; deletable wholesale when kratos v3 ships.
func Wrap(l *slog.Logger) kratoslog.Logger {
	if l == nil {
		l = slog.Default()
	}
	return &slogAdapter{logger: l}
}

// SetDefault updates the process-wide kratos v2 logger delegate.
//
// The kratos v2 global logger object is installed once by this package. Runtime
// code must update only this delegate, never call kratoslog.SetLogger again.
func SetDefault(l *slog.Logger) {
	defaultLogger.logger.Store(l)
}

func (a *slogAdapter) Log(level kratoslog.Level, keyvals ...any) error {
	msg, attrs := keyvalsToAttrs(keyvals)
	a.logger.LogAttrs(context.Background(), mapLevel(level), msg, attrs...)
	return nil
}

func (a *atomicLogger) Log(level kratoslog.Level, keyvals ...any) error {
	logger := a.logger.Load()
	if logger == nil {
		logger = slog.Default()
	}
	msg, attrs := keyvalsToAttrs(keyvals)
	logger.LogAttrs(context.Background(), mapLevel(level), msg, attrs...)
	return nil
}

func keyvalsToAttrs(keyvals []any) (string, []slog.Attr) {
	var msg string
	attrs := make([]slog.Attr, 0, len(keyvals)/2)
	for i := 0; i+1 < len(keyvals); i += 2 {
		k := fmt.Sprint(keyvals[i])
		v := keyvals[i+1]
		if k == kratoslog.DefaultMessageKey {
			msg = fmt.Sprint(v)
			continue
		}
		attrs = append(attrs, slog.Any(k, v))
	}
	return msg, attrs
}

func mapLevel(kl kratoslog.Level) slog.Level {
	switch kl {
	case kratoslog.LevelDebug:
		return slog.LevelDebug
	case kratoslog.LevelInfo:
		return slog.LevelInfo
	case kratoslog.LevelWarn:
		return slog.LevelWarn
	case kratoslog.LevelError, kratoslog.LevelFatal:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

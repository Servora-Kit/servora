package kratosv2

import (
	"context"
	"fmt"
	"log/slog"

	kratoslog "github.com/go-kratos/kratos/v2/log"
)

type slogAdapter struct {
	logger *slog.Logger
}

// Wrap adapts a *slog.Logger to kratos v2 log.Logger.
// Self-contained; deletable wholesale when kratos v3 ships.
func Wrap(l *slog.Logger) kratoslog.Logger {
	if l == nil {
		l = slog.Default()
	}
	return &slogAdapter{logger: l}
}

func (a *slogAdapter) Log(level kratoslog.Level, keyvals ...any) error {
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
	a.logger.LogAttrs(context.Background(), mapLevel(level), msg, attrs...)
	return nil
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

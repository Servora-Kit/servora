package logger

import (
	"context"
	"io"
	"log/slog"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

// buildFileHandler creates the file backend writer and returns a closer that
// releases the underlying lumberjack file handle. The closer is always non-nil
// when a non-nil handler is returned; callers MUST invoke it on shutdown so the
// log file is not leaked (which on Windows also blocks removal of the file).
func buildFileHandler(userFactory LogHandlerFunc, cfg *corev1.Log_FileBackend, lvl slog.Level) (slog.Handler, func(context.Context) error) {
	if cfg == nil || cfg.GetPath() == "" {
		return nil, nil
	}
	w := &lumberjack.Logger{
		Filename:   cfg.GetPath(),
		MaxSize:    int(cfg.GetMaxSize()),
		MaxBackups: int(cfg.GetMaxBackups()),
		MaxAge:     int(cfg.GetMaxAge()),
		Compress:   cfg.GetCompress(),
	}
	var h slog.Handler
	if userFactory != nil {
		h = userFactory(w, lvl)
	} else {
		h = defaultFileHandler(w, cfg, lvl)
	}
	return h, func(context.Context) error { return w.Close() }
}

func defaultFileHandler(w io.Writer, cfg *corev1.Log_FileBackend, lvl slog.Level) slog.Handler {
	if cfg.GetFormat() == corev1.Log_LOG_FORMAT_TEXT {
		cw := zerolog.ConsoleWriter{Out: w, NoColor: true}
		zl := zerolog.New(cw).Level(zerologLevel(lvl)).With().Timestamp().Logger()
		return zerolog.NewSlogHandler(zl)
	}
	zl := zerolog.New(w).Level(zerologLevel(lvl)).With().Timestamp().Logger()
	return zerolog.NewSlogHandler(zl)
}

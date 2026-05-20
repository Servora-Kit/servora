package logger

import (
	"io"
	"log/slog"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func buildFileHandler(userFactory LogHandlerFunc, cfg *corev1.Log_FileBackend, lvl slog.Level) slog.Handler {
	if cfg == nil || cfg.GetPath() == "" {
		return nil
	}
	w := &lumberjack.Logger{
		Filename:   cfg.GetPath(),
		MaxSize:    int(cfg.GetMaxSize()),
		MaxBackups: int(cfg.GetMaxBackups()),
		MaxAge:     int(cfg.GetMaxAge()),
		Compress:   cfg.GetCompress(),
	}
	if userFactory != nil {
		return userFactory(w, lvl)
	}
	return defaultFileHandler(w, cfg, lvl)
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

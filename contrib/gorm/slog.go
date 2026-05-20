package gorm

import (
	"log/slog"

	gormlogger "gorm.io/gorm/logger"
)

// NewSlogLogger creates a gorm logger backed by the given *slog.Logger.
// Uses gorm's built-in NewSlogLogger (caller fix #7610, v1.31.1+).
func NewSlogLogger(l *slog.Logger, cfg ...gormlogger.Config) gormlogger.Interface {
	c := gormlogger.Config{
		LogLevel: gormlogger.Warn,
	}
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return gormlogger.NewSlogLogger(l, c)
}

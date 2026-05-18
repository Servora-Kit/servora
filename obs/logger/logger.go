package logger

import (
	"log/slog"
	"sync/atomic"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	slogzap "github.com/samber/slog-zap/v2"
	"go.uber.org/zap"
)

var current atomic.Pointer[zap.Logger]

// NewHandler 装配默认 handler 栈：contextHandler → slog-zap(backend，复用 servora zap 定制)。
func NewHandler(app *corev1.App) slog.Handler {
	z := buildZap(app)
	current.Store(z)
	base := slogzap.Option{Logger: z}.NewZapHandler()
	return newContextHandler(base)
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

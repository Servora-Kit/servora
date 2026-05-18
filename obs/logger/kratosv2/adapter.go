// Package kratosv2 是 slog.Handler → kratos v2 log.Logger 的临时桥。
// kratos v3 期删除整个子包，slog.Handler 直接喂 log.NewLogger。
package kratosv2

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"
)

type adapter struct{ h slog.Handler }

// Logger 把 slog.Handler 适配成 kratos v2 log.Logger。
func Logger(h slog.Handler) kratoslog.Logger { return &adapter{h} }

func (a *adapter) Log(level kratoslog.Level, keyvals ...any) error {
	var lv slog.Level
	switch level {
	case kratoslog.LevelDebug:
		lv = slog.LevelDebug
	case kratoslog.LevelWarn:
		lv = slog.LevelWarn
	case kratoslog.LevelError, kratoslog.LevelFatal:
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	r := slog.NewRecord(timeNow(), lv, msgOf(keyvals), 0)
	for i := 0; i+1 < len(keyvals); i += 2 {
		if k, ok := keyvals[i].(string); ok && k != "msg" {
			r.Add(k, keyvals[i+1])
		}
	}
	return a.h.Handle(context.Background(), r)
}

func timeNow() time.Time { return time.Now() }

func msgOf(keyvals []any) string {
	for i := 0; i+1 < len(keyvals); i += 2 {
		if k, ok := keyvals[i].(string); ok && k == "msg" {
			return fmt.Sprint(keyvals[i+1])
		}
	}
	return ""
}

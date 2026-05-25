package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/term"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

// LogHandlerFunc is a factory that produces an slog.Handler writing to w.
// Servora calls it once per local backend (stdout / file), providing the
// appropriate io.Writer. OTel backend is independent and unaffected.
type LogHandlerFunc func(w io.Writer, level slog.Level) slog.Handler

// Option configures logger.New.
type Option func(*options)

type options struct {
	handlerFunc LogHandlerFunc
}

// WithLogHandlerFunc replaces the default zerolog encoding engine for
// stdout and file backends. The factory is called with the io.Writer
// that servora manages (os.Stdout or lumberjack). OTel is unaffected.
func WithLogHandlerFunc(f LogHandlerFunc) Option {
	return func(o *options) { o.handlerFunc = f }
}

// isTerminal is injectable for testing.
var isTerminal = func(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

// New assembles a logger from Bootstrap proto config.
// Returns a stdlib *slog.Logger and a closer (always non-nil; no-op
// when no OTel backend is active). Callers MUST invoke closer on shutdown.
func New(bc *corev1.Bootstrap, opts ...Option) (*slog.Logger, func(context.Context) error) {
	var o options
	for _, fn := range opts {
		fn(&o)
	}

	env := envOf(bc)
	lc := logCfgOf(bc)
	lvl := levelOf(lc)

	var (
		handlers []slog.Handler
		closers  []func(context.Context) error
	)

	for _, b := range lc.GetBackends() {
		switch x := b.GetBackend().(type) {
		case *corev1.Log_LogBackend_Stdout:
			handlers = append(handlers, buildStdoutHandler(o.handlerFunc, x.Stdout, env, lvl))
		case *corev1.Log_LogBackend_File:
			h := buildFileHandler(o.handlerFunc, x.File, lvl)
			if h != nil {
				handlers = append(handlers, h)
			}
		case *corev1.Log_LogBackend_Otel:
			h, closer := buildOtelHandler(x.Otel, lvl)
			if h != nil {
				handlers = append(handlers, h)
				closers = append(closers, closer)
			}
		case *corev1.Log_LogBackend_Noop:
			handlers = append(handlers, slog.DiscardHandler)
		}
	}

	if len(handlers) == 0 {
		handlers = append(handlers, buildStdoutHandler(o.handlerFunc, nil, env, lvl))
	}

	closer := func(ctx context.Context) error {
		var first error
		for _, c := range closers {
			if err := c(ctx); err != nil && first == nil {
				first = err
			}
		}
		return first
	}

	return slog.New(fanout(handlers)), closer
}

// --- stdout backend ---

func buildStdoutHandler(userFactory LogHandlerFunc, cfg *corev1.Log_StdoutBackend, env string, lvl slog.Level) slog.Handler {
	if userFactory != nil {
		return userFactory(os.Stdout, lvl)
	}
	return defaultStdoutHandler(cfg, env, lvl)
}

func defaultStdoutHandler(cfg *corev1.Log_StdoutBackend, env string, lvl slog.Level) slog.Handler {
	wantJSON := resolveStdoutJSON(cfg, env)
	if wantJSON {
		zl := zerolog.New(os.Stdout).Level(zerologLevel(lvl)).With().Timestamp().Logger()
		return zerolog.NewSlogHandler(zl)
	}
	useColor := isTerminal(os.Stdout.Fd())
	cw := zerolog.ConsoleWriter{Out: os.Stdout, NoColor: !useColor, TimeFormat: time.TimeOnly}
	zl := zerolog.New(cw).Level(zerologLevel(lvl)).With().Timestamp().Logger()
	return zerolog.NewSlogHandler(zl)
}

func resolveStdoutJSON(cfg *corev1.Log_StdoutBackend, env string) bool {
	if cfg != nil {
		switch cfg.GetFormat() {
		case corev1.Log_LOG_FORMAT_JSON:
			return true
		case corev1.Log_LOG_FORMAT_TEXT:
			return false
		}
	}
	return env != "dev" && env != ""
}

// --- helpers ---

func envOf(bc *corev1.Bootstrap) string {
	if bc == nil || bc.GetApp() == nil || bc.GetApp().GetEnv() == "" {
		return "dev"
	}
	return bc.GetApp().GetEnv()
}

func logCfgOf(bc *corev1.Bootstrap) *corev1.Log {
	if bc == nil || bc.GetObs().GetLog() == nil {
		return &corev1.Log{}
	}
	return bc.GetObs().GetLog()
}

func levelOf(lc *corev1.Log) slog.Level {
	switch lc.GetLevel() {
	case corev1.Log_LOG_LEVEL_DEBUG:
		return slog.LevelDebug
	case corev1.Log_LOG_LEVEL_WARN:
		return slog.LevelWarn
	case corev1.Log_LOG_LEVEL_ERROR:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func zerologLevel(lvl slog.Level) zerolog.Level {
	switch {
	case lvl <= slog.LevelDebug:
		return zerolog.DebugLevel
	case lvl <= slog.LevelInfo:
		return zerolog.InfoLevel
	case lvl <= slog.LevelWarn:
		return zerolog.WarnLevel
	default:
		return zerolog.ErrorLevel
	}
}

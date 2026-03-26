package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var _ log.Logger = (*ZapLogger)(nil)

// Logger and Helper are type aliases for Kratos interfaces, kept for ergonomics.
type Logger = log.Logger
type Helper = log.Helper

// ZapLogger wraps *zap.Logger and satisfies kratos log.Logger.
type ZapLogger struct {
	log *zap.Logger
}

// New creates a ZapLogger from a proto App config. nil-safe: returns a dev
// console logger when app is nil or app.Log is nil.
func New(app *conf.App) *ZapLogger {
	env := "dev"
	var logCfg *conf.App_Log
	if app != nil {
		env = app.GetEnv()
		logCfg = app.GetLog()
	}

	filename := ""
	var lj *lumberjack.Logger
	if logCfg != nil {
		filename = logCfg.GetFilename()
		if filename == "" {
			filename = "./logs/app.log"
		}
		if dir := filepath.Dir(filename); dir != "." && dir != "/" {
			_ = os.MkdirAll(dir, 0755)
		}
		maxSize := int(logCfg.GetMaxSize())
		if maxSize == 0 {
			maxSize = 10
		}
		maxBackups := int(logCfg.GetMaxBackups())
		if maxBackups == 0 {
			maxBackups = 5
		}
		maxAge := int(logCfg.GetMaxAge())
		if maxAge == 0 {
			maxAge = 30
		}
		lj = &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   logCfg.GetCompress(),
		}
	}

	level := kratoLevelToZap(logCfg.GetLevel())
	atomicLevel := zap.NewAtomicLevelAt(level)

	var core zapcore.Core
	switch env {
	case "dev":
		enc := zap.NewDevelopmentEncoderConfig()
		enc.EncodeTime = zapcore.ISO8601TimeEncoder
		enc.EncodeLevel = zapcore.CapitalColorLevelEncoder
		core = zapcore.NewCore(zapcore.NewConsoleEncoder(enc), zapcore.AddSync(os.Stdout), atomicLevel)
	case "test":
		core = zapcore.NewNopCore()
	default:
		core = buildProdCore(atomicLevel, lj)
	}

	opts := []zap.Option{
		zap.AddStacktrace(zap.NewAtomicLevelAt(zapcore.ErrorLevel)),
		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.Development(),
	}
	return &ZapLogger{log: zap.New(core, opts...)}
}

// buildProdCore builds a prod/default tee core (console + optional file).
func buildProdCore(level zap.AtomicLevel, lj *lumberjack.Logger) zapcore.Core {
	enc := zap.NewProductionEncoderConfig()
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	console := zapcore.NewCore(zapcore.NewConsoleEncoder(enc), zapcore.AddSync(os.Stdout), level)
	if lj == nil {
		return console
	}
	file := zapcore.NewCore(zapcore.NewJSONEncoder(enc), zapcore.AddSync(lj), level)
	return zapcore.NewTee(console, file)
}

// kratoLevelToZap maps Kratos log level int32 to zapcore.Level.
func kratoLevelToZap(l int32) zapcore.Level {
	switch l {
	case 0:
		return zapcore.DebugLevel
	case 1:
		return zapcore.InfoLevel
	case 2:
		return zapcore.WarnLevel
	case 3:
		return zapcore.ErrorLevel
	case 4:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Zap returns the underlying *zap.Logger for integrations that require it
// (e.g. franz-go kzap plugin, GORM bridge).
func (l *ZapLogger) Zap() *zap.Logger { return l.log }

// Sync flushes any buffered log entries.
func (l *ZapLogger) Sync() error { return l.log.Sync() }

// Log implements kratos log.Logger.
func (l *ZapLogger) Log(level log.Level, keyvals ...any) error {
	if len(keyvals) == 0 || len(keyvals)%2 != 0 {
		l.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
		return nil
	}
	fields := make([]zap.Field, 0, len(keyvals)/2)
	for i := 0; i < len(keyvals); i += 2 {
		fields = append(fields, zap.Any(fmt.Sprint(keyvals[i]), keyvals[i+1]))
	}
	switch level {
	case log.LevelDebug:
		l.log.Debug("", fields...)
	case log.LevelInfo:
		l.log.Info("", fields...)
	case log.LevelWarn:
		l.log.Warn("", fields...)
	case log.LevelError:
		l.log.Error("", fields...)
	case log.LevelFatal:
		l.log.Fatal("", fields...)
	}
	return nil
}

// GetGormLogger returns a GORM-compatible logger scoped to the given module.
func (l *ZapLogger) GetGormLogger(module string) GormLogger {
	return GormLogger{
		ZapLogger:     l.Zap().With(zap.String("module", module)),
		SlowThreshold: 200 * time.Millisecond,
	}
}

// ── Option helpers ────────────────────────────────────────────────────────────

// Option is a function that appends key-value pairs to a logger's fields.
type Option func(keyvals *[]any)

// WithModule returns an Option that adds a "module" field.
// Naming convention: "component/layer/service" (e.g. "user/biz/iam").
func WithModule(module string) Option {
	return func(kv *[]any) { *kv = append(*kv, "module", module) }
}

// WithField returns an Option that adds an arbitrary key-value field.
func WithField(key string, value any) Option {
	return func(kv *[]any) { *kv = append(*kv, key, value) }
}

// With adds structured fields to a logger. Accepts two styles:
//
//	logger.With(l, "component/layer/service")          — module shorthand
//	logger.With(l, WithModule("x"), WithField("k", v)) — option style
func With(l Logger, args ...any) Logger {
	kv := make([]any, 0, len(args)*2)
	for _, arg := range args {
		switch x := arg.(type) {
		case string:
			kv = append(kv, "module", x)
		case Option:
			x(&kv)
		}
	}
	return log.With(l, kv...)
}

// For creates a *Helper scoped to the given module — the one-liner replacement
// for logger.NewHelper(l, logger.WithModule("x/y/z")).
//
//	Before: logger.NewHelper(l, logger.WithModule("user/biz/iam-service"))
//	After:  logger.For(l, "user/biz/iam")
func For(l Logger, module string) *Helper {
	return log.NewHelper(log.With(l, "module", module))
}

// NewHelper creates a *Helper with optional Option fields applied.
// Prefer For() when only a module label is needed.
func NewHelper(l Logger, opts ...Option) *Helper {
	if len(opts) > 0 {
		kv := make([]any, 0, len(opts)*2)
		for _, opt := range opts {
			opt(&kv)
		}
		l = log.With(l, kv...)
	}
	return log.NewHelper(l)
}

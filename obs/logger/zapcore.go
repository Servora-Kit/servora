package logger

import (
	"os"
	"path/filepath"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// buildZap assembles a *zap.Logger from a proto App config, reusing the
// original New() zap wiring: lumberjack rolling files, prod/dev/test core
// switching, and Kratos-level parsing. It is nil-safe: a nil app (or nil
// app.Log) yields a dev console logger. The bare *zap.Logger is returned
// without the kratos log.Logger wrapper — convenience/helper layers and the
// GORM bridge live elsewhere.
func buildZap(app *corev1.App) *zap.Logger {
	env := "dev"
	var logCfg *corev1.App_Log
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
	return zap.New(core, opts...)
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

package logger

import (
	"os"
	"testing"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	gormlogger "gorm.io/gorm/logger"
)

func TestGormLoggerFrom_ZapLogger(t *testing.T) {
	base := New(nil)

	gormLog := GormLoggerFrom(base, "gorm/test/servora")
	if gormLog == nil {
		t.Fatal("expected non-nil gorm logger")
	}

	if _, ok := gormLog.(GormLogger); !ok {
		t.Fatalf("expected GormLogger when input is ZapLogger, got %T", gormLog)
	}
}

func TestGormLoggerFrom_Fallback(t *testing.T) {
	std := kratoslog.NewStdLogger(os.Stdout)

	gormLog := GormLoggerFrom(std, "gorm/test/servora")
	if gormLog == nil {
		t.Fatal("expected non-nil fallback gorm logger")
	}

	if _, ok := gormLog.(GormLogger); ok {
		t.Fatalf("expected fallback logger for non-Zap input, got %T", gormLog)
	}

	if got := gormLog.LogMode(gormlogger.Warn); got == nil {
		t.Fatal("expected fallback logger to support LogMode")
	}
}

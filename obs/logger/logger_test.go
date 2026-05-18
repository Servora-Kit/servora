package logger

import (
	"log/slog"
	"testing"
)

func TestNewLogger_NilSafe(t *testing.T) {
	if NewLogger(nil) == nil {
		t.Fatal("NewLogger(nil) should be non-nil")
	}
	var _ slog.Handler = NewHandler(nil)
}

func TestSync_NoPanic(t *testing.T) {
	_ = NewLogger(nil)
	if err := Sync(); err != nil {
		t.Logf("sync non-fatal: %v", err)
	}
}

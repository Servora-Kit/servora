package logger

import (
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

func TestNew_NilSafe(t *testing.T) {
	l := New(nil)
	if l == nil {
		t.Fatal("New(nil) must not return nil")
	}
	if err := l.Log(0, "msg", "nil-safe"); err != nil {
		t.Fatalf("Log on nil-app logger: %v", err)
	}
}

func TestNew_WithConfig(t *testing.T) {
	app := &conf.App{
		Env: "test",
		Log: &conf.App_Log{Filename: "/tmp/test-logger.log"},
	}
	l := New(app)
	if l == nil {
		t.Fatal("New(app) must not return nil")
	}
}

func TestNew_DefaultFilename(t *testing.T) {
	// When filename is empty, New picks the default internally (not mutating proto).
	// We just verify it does not panic and returns a usable logger.
	app := &conf.App{Env: "test", Log: &conf.App_Log{}}
	l := New(app)
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_ZapGetter(t *testing.T) {
	l := New(nil)
	if l.Zap() == nil {
		t.Fatal("Zap() must return non-nil *zap.Logger")
	}
}

func TestNew_SyncMethod(t *testing.T) {
	l := New(nil)
	// Sync() is now a method, not a field; call must not panic.
	_ = l.Sync()
}

func TestFor(t *testing.T) {
	l := New(nil)
	h := For(l, "user/biz/iam")
	if h == nil {
		t.Fatal("For() must return non-nil *Helper")
	}
	h.Info("For helper works")
}

func TestWith_StringShorthand(t *testing.T) {
	l := New(nil)
	wl := With(l, "http/server/iam")
	if wl == nil {
		t.Fatal("With(l, module) must return non-nil Logger")
	}
}

func TestWith_OptionStyle(t *testing.T) {
	l := New(nil)
	wl := With(l, WithModule("component/layer"), WithField("op", "test"))
	if wl == nil {
		t.Fatal("With(l, opts...) must return non-nil Logger")
	}
}

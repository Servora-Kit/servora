package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/go-kratos/kratos/v2"
)

func TestRunWithRuntime_ValidateInput(t *testing.T) {
	t.Run("nil runtime", func(t *testing.T) {
		err := defaultRunner.runWithRuntime(nil, func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
			return nil, nil, nil
		})
		if err == nil {
			t.Fatal("expected error for nil runtime")
		}
	})

	t.Run("nil builder", func(t *testing.T) {
		err := defaultRunner.runWithRuntime(&Runtime{}, nil)
		if err == nil {
			t.Fatal("expected error for nil builder")
		}
	})
}

func TestRunWithRuntime_BuilderError(t *testing.T) {
	want := errors.New("build failed")
	err := defaultRunner.runWithRuntime(&Runtime{}, func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
		return nil, nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRunWithRuntime_RunAndCleanup(t *testing.T) {
	calledRun := false
	calledCleanup := false
	want := errors.New("run failed")

	runner := newRunner(nil, func(_ *kratos.App) error {
		calledRun = true
		return want
	})

	err := runner.runWithRuntime(&Runtime{}, func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
		return &kratos.App{}, func() { calledCleanup = true }, nil
	})

	if !calledRun {
		t.Fatal("runApp should be called")
	}
	if !calledCleanup {
		t.Fatal("cleanup should be called")
	}
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRunWithRuntime_NilAppShouldFailFast(t *testing.T) {
	calledRun := false
	runner := newRunner(nil, func(_ *kratos.App) error {
		calledRun = true
		return nil
	})

	err := runner.runWithRuntime(&Runtime{}, func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
		return nil, nil, nil
	})

	if err == nil {
		t.Fatal("expected error when app is nil")
	}
	if calledRun {
		t.Fatal("runApp should not be called when app is nil")
	}
}

func TestBootstrapAndRun_CloseRuntime(t *testing.T) {
	closed := false
	runner := newRunner(func(_, _, _ string, _ bootstrapOptions) (*Runtime, error) {
		return &Runtime{
			configCloser: func() { closed = true },
		}, nil
	}, func(_ *kratos.App) error { return nil })

	err := runner.bootstrapAndRun("/tmp/configs", "svc", "v1", func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
		return &kratos.App{}, nil, nil
	})
	if err != nil {
		t.Fatalf("BootstrapAndRun error = %v", err)
	}
	if !closed {
		t.Fatal("runtime close should be called")
	}
}

func TestNewRunner_UsesDefaultWhenNil(t *testing.T) {
	runner := newRunner(nil, nil)
	if runner.newRuntime == nil {
		t.Fatal("newRuntime should fallback to default")
	}
	if runner.runApp == nil {
		t.Fatal("runApp should fallback to default")
	}
}

func TestBootstrapAndRun_EmitStageLogs(t *testing.T) {
	cl := &captureLogger{}
	runner := newRunner(func(_, _, _ string, _ bootstrapOptions) (*Runtime, error) {
		return &Runtime{Logger: slog.New(cl)}, nil
	}, func(_ *kratos.App) error { return nil })

	err := runner.bootstrapAndRun("/tmp/configs", "svc", "v1", func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
		return &kratos.App{}, nil, nil
	})
	if err != nil {
		t.Fatalf("bootstrapAndRun error = %v", err)
	}

	if !cl.hasStage("bootstrap_start") {
		t.Fatal("missing bootstrap_start stage log")
	}
	if !cl.hasStage("run_with_runtime_start") {
		t.Fatal("missing run_with_runtime_start stage log")
	}
	if !cl.hasStage("run_with_runtime_done") {
		t.Fatal("missing run_with_runtime_done stage log")
	}
	if !cl.hasStage("bootstrap_done") {
		t.Fatal("missing bootstrap_done stage log")
	}
}

// captureLogger 是一个最小 slog.Handler，记录每条日志的 message（即 stage 名）。
type captureLogger struct {
	stages []string
}

func (c *captureLogger) Enabled(context.Context, slog.Level) bool { return true }

func (c *captureLogger) Handle(_ context.Context, r slog.Record) error {
	c.stages = append(c.stages, r.Message)
	return nil
}

func (c *captureLogger) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c *captureLogger) WithGroup(string) slog.Handler { return c }

func (c *captureLogger) hasStage(stage string) bool {
	for _, s := range c.stages {
		if s == stage {
			return true
		}
	}
	return false
}

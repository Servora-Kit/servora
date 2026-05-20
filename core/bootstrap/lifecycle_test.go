package bootstrap

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
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
	var buf bytes.Buffer
	sl := slog.New(slog.NewTextHandler(&buf, nil))
	runner := newRunner(func(_, _, _ string, _ bootstrapOptions) (*Runtime, error) {
		return &Runtime{Logger: sl}, nil
	}, func(_ *kratos.App) error { return nil })

	err := runner.bootstrapAndRun("/tmp/configs", "svc", "v1", func(_ *Runtime) (app *kratos.App, cleanup func(), err error) {
		return &kratos.App{}, nil, nil
	})
	if err != nil {
		t.Fatalf("bootstrapAndRun error = %v", err)
	}

	output := buf.String()
	for _, stage := range []string{"bootstrap_start", "run_with_runtime_start", "run_with_runtime_done", "bootstrap_done"} {
		if !strings.Contains(output, stage) {
			t.Fatalf("missing stage log: %s\noutput: %s", stage, output)
		}
	}
}

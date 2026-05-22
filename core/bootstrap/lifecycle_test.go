package bootstrap

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/go-kratos/kratos/v2"
)

func TestRuntimeClose_LIFOOnceAndSameError(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	var order []string
	rt := &Runtime{cleanups: []func(context.Context) error{
		func(context.Context) error { order = append(order, "config"); return firstErr },
		func(context.Context) error { order = append(order, "log"); return nil },
		func(context.Context) error { order = append(order, "trace"); return secondErr },
	}}

	err := rt.Close(context.Background())
	if !reflect.DeepEqual(order, []string{"trace", "log", "config"}) {
		t.Fatalf("cleanup order = %v, want LIFO", order)
	}
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("error = %v, want joined cleanup errors", err)
	}

	again := rt.Close(context.Background())
	if again != err {
		t.Fatalf("second Close returned different error instance: %v vs %v", again, err)
	}
	if !reflect.DeepEqual(order, []string{"trace", "log", "config"}) {
		t.Fatalf("cleanup order after second close = %v, want unchanged", order)
	}
}

func TestRuntimeClose_RecoversPanic(t *testing.T) {
	rt := &Runtime{cleanups: []func(context.Context) error{
		func(context.Context) error { panic("cleanup panic") },
	}}

	err := rt.Close(context.Background())
	if err == nil || err.Error() != "panic: cleanup panic" {
		t.Fatalf("error = %v, want recovered panic", err)
	}
}

func TestRuntimeClose_ChecksContextBetweenCleanups(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var order []string
	rt := &Runtime{cleanups: []func(context.Context) error{
		func(context.Context) error { order = append(order, "config"); return nil },
		func(context.Context) error { order = append(order, "log"); return nil },
		func(context.Context) error { order = append(order, "trace"); cancel(); return nil },
	}}

	err := rt.Close(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if !reflect.DeepEqual(order, []string{"trace"}) {
		t.Fatalf("cleanup order = %v, want only first cleanup before context cancellation", order)
	}
}

func TestRuntimeClose_NilRuntime(t *testing.T) {
	var rt *Runtime
	if err := rt.Close(context.Background()); err != nil {
		t.Fatalf("nil Runtime Close error = %v, want nil", err)
	}
}

func TestRuntimeNewApp_DefaultsAndCallerOverrides(t *testing.T) {
	metadata := map[string]string{"zone": "cn-east"}
	rt := &Runtime{
		Bootstrap: &corev1.Bootstrap{App: &corev1.App{
			Name:     "svc.service",
			Version:  "v1.0.0",
			Metadata: metadata,
		}},
		serviceID: "svc.service-node-a",
	}

	app := rt.NewApp(kratos.Name("override.service"))
	if app.ID() != "svc.service-node-a" {
		t.Fatalf("app.ID = %q, want svc.service-node-a", app.ID())
	}
	if app.Name() != "override.service" {
		t.Fatalf("app.Name = %q, want caller override", app.Name())
	}
	if app.Version() != "v1.0.0" {
		t.Fatalf("app.Version = %q, want v1.0.0", app.Version())
	}
	if app.Metadata()["zone"] != "cn-east" {
		t.Fatalf("app.Metadata zone = %q, want cn-east", app.Metadata()["zone"])
	}
}

func TestRuntimeRun_BuildErrorStillClosesRuntime(t *testing.T) {
	buildErr := errors.New("build failed")
	closeErr := errors.New("close failed")
	var order []string
	rt := &Runtime{cleanups: []func(context.Context) error{
		func(context.Context) error { order = append(order, "runtime"); return closeErr },
	}}

	err := rt.Run(func() (*kratos.App, func(), error) {
		return nil, nil, buildErr
	})
	if !errors.Is(err, buildErr) || !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want build and close errors", err)
	}
	if !reflect.DeepEqual(order, []string{"runtime"}) {
		t.Fatalf("cleanup order = %v, want runtime close", order)
	}
}

func TestRuntimeRun_CleanupBeforeRuntimeClose(t *testing.T) {
	var order []string
	rt := &Runtime{
		Bootstrap: &corev1.Bootstrap{App: &corev1.App{
			Name:     "svc.service",
			Version:  "v1.0.0",
			Metadata: map[string]string{},
		}},
		serviceID: "svc.service-node-a",
		cleanups: []func(context.Context) error{
			func(context.Context) error { order = append(order, "runtime"); return nil },
		},
	}

	var app *kratos.App
	app = rt.NewApp(kratos.BeforeStart(func(context.Context) error {
		go func() { _ = app.Stop() }()
		return nil
	}))

	done := make(chan error, 1)
	go func() {
		done <- rt.Run(func() (*kratos.App, func(), error) {
			return app, func() { order = append(order, "business") }, nil
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run error = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}

	if !reflect.DeepEqual(order, []string{"business", "runtime"}) {
		t.Fatalf("cleanup order = %v, want business cleanup before runtime close", order)
	}
}

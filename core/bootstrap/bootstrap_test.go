package bootstrap

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	klog "github.com/go-kratos/kratos/v3/log"
)

func TestResolveAppIdentity_ConfigValuesWin(t *testing.T) {
	app := &corev1.App{
		Name:     "custom.service",
		Version:  "v9.9.9",
		Metadata: map[string]string{"zone": "cn-east"},
	}

	if err := resolveAppIdentity(app, "default.service", "v1.0.0"); err != nil {
		t.Fatalf("resolveAppIdentity() error = %v", err)
	}

	if app.Name != "custom.service" {
		t.Fatalf("app.name = %q, want %q", app.Name, "custom.service")
	}
	if app.Version != "v9.9.9" {
		t.Fatalf("app.version = %q, want %q", app.Version, "v9.9.9")
	}
	if app.Metadata["zone"] != "cn-east" {
		t.Fatalf("metadata[zone] = %q, want %q", app.Metadata["zone"], "cn-east")
	}
}

func TestResolveAppIdentity_OptionsFillEmptyValues(t *testing.T) {
	app := &corev1.App{}

	if err := resolveAppIdentity(app, "default.service", "v1.0.0"); err != nil {
		t.Fatalf("resolveAppIdentity() error = %v", err)
	}

	if app.Name != "default.service" {
		t.Fatalf("app.name = %q, want %q", app.Name, "default.service")
	}
	if app.Version != "v1.0.0" {
		t.Fatalf("app.version = %q, want %q", app.Version, "v1.0.0")
	}
	if app.Metadata == nil {
		t.Fatal("app.metadata should be initialized")
	}
}

func TestResolveAppIdentity_RequiresNameAndVersion(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		err := resolveAppIdentity(&corev1.App{Version: "v1"}, "", "")
		if err == nil || err.Error() != "bootstrap: app name is required" {
			t.Fatalf("error = %v, want app name required", err)
		}
	})

	t.Run("missing version", func(t *testing.T) {
		err := resolveAppIdentity(&corev1.App{Name: "svc"}, "", "")
		if err == nil || err.Error() != "bootstrap: app version is required" {
			t.Fatalf("error = %v, want app version required", err)
		}
	})
}

func TestNewRuntime_OptionsFillConfigAndServiceID(t *testing.T) {
	configFile := writeBootstrapConfig(t, "app:\n  env: dev\n")
	withHostname(t, "node-a", nil)

	rt, err := NewRuntime(configFile, Name("default.service"), Version("v1.0.0"))
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer func() { _ = rt.Close(t.Context()) }()

	if rt.Bootstrap.App.Name != "default.service" {
		t.Fatalf("app.name = %q, want default.service", rt.Bootstrap.App.Name)
	}
	if rt.Bootstrap.App.Version != "v1.0.0" {
		t.Fatalf("app.version = %q, want v1.0.0", rt.Bootstrap.App.Version)
	}
	if rt.serviceID != "default.service-node-a" {
		t.Fatalf("serviceID = %q, want default.service-node-a", rt.serviceID)
	}
	if rt.Bootstrap.App.Metadata == nil {
		t.Fatal("app.metadata should be initialized")
	}
	if rt.Logger == nil {
		t.Fatal("Logger should be initialized")
	}
	if len(rt.cleanups) != 3 {
		t.Fatalf("cleanups = %d, want 3", len(rt.cleanups))
	}
}

func TestNewRuntime_BindsKratosDefaultLogger(t *testing.T) {
	old := klog.Default()
	t.Cleanup(func() { klog.SetDefault(old) })

	var buf bytes.Buffer

	configFile := writeBootstrapConfig(t, "app:\n  env: dev\n")
	withHostname(t, "node-a", nil)

	rt, err := NewRuntime(
		configFile,
		Name("default.service"),
		Version("v1.0.0"),
		WithLogHandlerFunc(func(_ io.Writer, lvl slog.Level) slog.Handler {
			return slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: lvl})
		}),
	)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer func() { _ = rt.Close(t.Context()) }()

	if klog.Default() != rt.Logger {
		t.Fatal("expected Kratos default logger to be runtime logger")
	}

	rt.NewApp()
	if klog.Default() != rt.Logger {
		t.Fatal("Runtime.NewApp should not rewrite Kratos default logger")
	}

	klog.Info("kratos-default-visible")
	if got := buf.String(); !strings.Contains(got, "kratos-default-visible") || !strings.Contains(got, "service=default.service") {
		t.Fatalf("Kratos default log output = %q, want message and service field", got)
	}
}

func TestNewRuntime_ConfigValuesWinOverOptions(t *testing.T) {
	configFile := writeBootstrapConfig(t, "app:\n  name: config.service\n  version: v2.0.0\n")
	withHostname(t, "node-b", nil)

	rt, err := NewRuntime(configFile, Name("default.service"), Version("v1.0.0"))
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	defer func() { _ = rt.Close(t.Context()) }()

	if rt.Bootstrap.App.Name != "config.service" {
		t.Fatalf("app.name = %q, want config.service", rt.Bootstrap.App.Name)
	}
	if rt.Bootstrap.App.Version != "v2.0.0" {
		t.Fatalf("app.version = %q, want v2.0.0", rt.Bootstrap.App.Version)
	}
	if rt.serviceID != "config.service-node-b" {
		t.Fatalf("serviceID = %q, want config.service-node-b", rt.serviceID)
	}
}

func TestNewRuntime_WithEnvPrefixRequiresName(t *testing.T) {
	_, err := NewRuntime("does-not-matter.yaml", WithEnvPrefix())
	if err == nil || err.Error() != "bootstrap: WithEnvPrefix requires Name option" {
		t.Fatalf("error = %v, want WithEnvPrefix requires Name option", err)
	}
}

func TestNewRuntime_HostnameError(t *testing.T) {
	configFile := writeBootstrapConfig(t, "app:\n  name: svc.service\n  version: v1\n")
	want := errors.New("boom")
	withHostname(t, "", want)

	_, err := NewRuntime(configFile)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapping %v", err, want)
	}
	if err == nil || err.Error() != "bootstrap: hostname: boom" {
		t.Fatalf("error = %v, want hostname wrap", err)
	}
}

func writeBootstrapConfig(t *testing.T, content string) string {
	t.Helper()
	configFile := filepath.Join(t.TempDir(), "bootstrap.yaml")
	if err := os.WriteFile(configFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return configFile
}

func withHostname(t *testing.T, hostname string, err error) {
	t.Helper()
	old := hostnameFn
	hostnameFn = func() (string, error) { return hostname, err }
	t.Cleanup(func() { hostnameFn = old })
}

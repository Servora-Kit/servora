package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

func TestResolveServiceIdentity_UseConfigValues(t *testing.T) {
	app := &conf.App{
		Name:     "custom.service",
		Version:  "v9.9.9",
		Metadata: map[string]string{"zone": "cn-east"},
	}

	meta := resolveServiceIdentity("default.service", "v1.0.0", "node-a", app)

	if meta.Name != "custom.service" {
		t.Fatalf("name = %q, want %q", meta.Name, "custom.service")
	}
	if meta.Version != "v9.9.9" {
		t.Fatalf("version = %q, want %q", meta.Version, "v9.9.9")
	}
	if meta.ID != "custom.service-node-a" {
		t.Fatalf("id = %q, want %q", meta.ID, "custom.service-node-a")
	}
	if meta.Metadata["zone"] != "cn-east" {
		t.Fatalf("metadata[zone] = %q, want %q", meta.Metadata["zone"], "cn-east")
	}
}

func TestResolveServiceIdentity_DefaultsAndMutatesApp(t *testing.T) {
	app := &conf.App{}

	meta := resolveServiceIdentity("default.service", "v1.0.0", "node-b", app)

	if meta.Name != "default.service" {
		t.Fatalf("name = %q, want %q", meta.Name, "default.service")
	}
	if meta.Version != "v1.0.0" {
		t.Fatalf("version = %q, want %q", meta.Version, "v1.0.0")
	}
	if meta.ID != "default.service-node-b" {
		t.Fatalf("id = %q, want %q", meta.ID, "default.service-node-b")
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

func TestScanConf(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "bootstrap.yaml")
	content := []byte(`
seed:
  admin_name: "root-admin"
`)
	if err := os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg := kconfig.New(
		kconfig.WithSource(file.NewSource(configFile)),
		kconfig.WithResolveActualTypes(true),
	)
	if err := cfg.Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	t.Cleanup(func() { _ = cfg.Close() })

	rt := &Runtime{Config: cfg}

	type Biz struct {
		Seed struct {
			AdminName string `json:"admin_name"`
		} `json:"seed"`
	}
	biz, err := ScanConf[Biz](rt)
	if err != nil {
		t.Fatalf("ScanConf failed: %v", err)
	}
	if biz.Seed.AdminName != "root-admin" {
		t.Fatalf("admin_name = %q, want %q", biz.Seed.AdminName, "root-admin")
	}
}

func TestScanConf_NilRuntime(t *testing.T) {
	type Any struct{}
	if _, err := ScanConf[Any](nil); err == nil {
		t.Fatal("ScanConf() error = nil, want error")
	}
}

package bootstrap

import (
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
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

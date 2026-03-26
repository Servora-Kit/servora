package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBootstrap(t *testing.T) {
	t.Setenv("SVC_APP_NAME", "from-env")

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  name: from-file\n"), 0o600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	bc, cfg, err := LoadBootstrap(configPath, "svc.service", false)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer func() { _ = cfg.Close() }()

	if bc == nil || bc.App == nil {
		t.Fatalf("LoadBootstrap() returned nil bootstrap/app")
	}
	if bc.App.Name != "from-file" {
		t.Fatalf("LoadBootstrap() app.name = %q, want %q", bc.App.Name, "from-file")
	}
}

func TestLoadBootstrapFromDirectory(t *testing.T) {
	t.Setenv("SVC_APP_NAME", "from-env")

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "bootstrap.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  name: from-dir\n"), 0o600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	bc, cfg, err := LoadBootstrap(configDir, "svc.service", false)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer func() { _ = cfg.Close() }()

	if bc == nil || bc.App == nil {
		t.Fatalf("LoadBootstrap() returned nil bootstrap/app")
	}
	if bc.App.Name != "from-dir" {
		t.Fatalf("LoadBootstrap() app.name = %q, want %q", bc.App.Name, "from-dir")
	}
}

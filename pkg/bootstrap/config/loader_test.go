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

	bc, cfg, err := LoadBootstrap(configPath, "svc.service")
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer cfg.Close()

	if bc == nil || bc.App == nil {
		t.Fatalf("LoadBootstrap() returned nil bootstrap/app")
	}
	if bc.App.Name != "from-file" {
		t.Fatalf("LoadBootstrap() app.name = %q, want %q", bc.App.Name, "from-file")
	}
}

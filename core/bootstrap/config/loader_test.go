package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBootstrap(t *testing.T) {
	t.Setenv("SVC_APP_NAME", "from-env")

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  name: from-file\nserver:\n  http:\n    listen:\n      addr: \":8080\"\n  grpc:\n    listen:\n      addr: \":9090\"\n"), 0o600); err != nil {
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
	if err := os.WriteFile(configPath, []byte("app:\n  name: from-dir\nserver:\n  http:\n    listen:\n      addr: \":8080\"\n  grpc:\n    listen:\n      addr: \":9090\"\n"), 0o600); err != nil {
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

func TestLoadBootstrap_AppliesProtoDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  name: x\nserver:\n  http:\n    listen:\n      addr: \":8080\"\n  grpc:\n    listen:\n      addr: \":9090\"\n"), 0o600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	bc, cfg, err := LoadBootstrap(configPath, "svc.service", false)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer func() { _ = cfg.Close() }()

	if got := bc.GetServer().GetHttp().GetListen().GetNetwork(); got != "tcp" {
		t.Fatalf("server.http.listen.network = %q, want %q", got, "tcp")
	}
	if got := bc.GetServer().GetHttp().GetListen().GetAddr(); got != ":8080" {
		t.Fatalf("server.http.listen.addr = %q, want %q", got, ":8080")
	}
	if bc.GetSource() != nil {
		t.Fatalf("bc.Source = %v, want nil", bc.GetSource())
	}
}

func TestLoadBootstrap_RequiredFieldMissing(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  name: x\nserver:\n  http:\n    listen:\n      network: tcp4\n"), 0o600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	_, _, err := LoadBootstrap(configPath, "svc.service", false)
	if err == nil {
		t.Fatal("LoadBootstrap should fail when server.http.listen.addr is missing")
	}
	if !strings.Contains(err.Error(), "addr is required") {
		t.Fatalf("error should mention addr, got: %v", err)
	}
}

func TestLoadBootstrap_NoServerSection(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  name: x\n"), 0o600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	bc, cfg, err := LoadBootstrap(configPath, "svc.service", false)
	if err != nil {
		t.Fatalf("LoadBootstrap() should succeed with no server section, got: %v", err)
	}
	defer func() { _ = cfg.Close() }()

	if got := bc.GetServer().GetHttp().GetListen().GetNetwork(); got != "tcp" {
		t.Fatalf("network = %q, want %q (default after ApplyDefaults)", got, "tcp")
	}
}

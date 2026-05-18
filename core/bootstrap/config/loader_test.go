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

func TestLoadBootstrap_AppliesProtoDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	// 完全没有 server: 段
	if err := os.WriteFile(configPath, []byte("app:\n  name: x\n"), 0o600); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	bc, cfg, err := LoadBootstrap(configPath, "svc.service", false)
	if err != nil {
		t.Fatalf("LoadBootstrap() error = %v", err)
	}
	defer func() { _ = cfg.Close() }()

	// allocate-then-default：缺配的注解字段被填默认
	if got := bc.GetServer().GetHttp().GetListen().GetNetwork(); got != "tcp" {
		t.Fatalf("server.http.listen.network = %q, want %q", got, "tcp")
	}
	// 无注解字段（addr 仅 required 无 default）仍为零值
	if got := bc.GetServer().GetHttp().GetListen().GetAddr(); got != "" {
		t.Fatalf("server.http.listen.addr = %q, want \"\" (无 default 注解)", got)
	}
	// 配置中心检测不受影响：未配置 → Config 仍 nil
	if bc.GetConfig() != nil {
		t.Fatalf("bc.Config = %v, want nil（门控天然不分配配置中心 oneof）", bc.GetConfig())
	}
}

package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	redispb "github.com/Servora-Kit/servora/api/gen/go/servora/contrib/db/redis/v1"
	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	apidocsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/apidocs/v1"
	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
	kconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

// loadKratosConfig 创建独立的临时来源并在测试结束时关闭。
func loadKratosConfig(t *testing.T, yaml string) kconfig.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := kconfig.New(kconfig.WithSource(file.NewSource(path)), kconfig.WithResolveActualTypes(true))
	if err := cfg.Load(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cfg.Close() })
	return cfg
}

func TestScanRejectsInvalidRuntimeAndTarget(t *testing.T) {
	if err := Scan(nil); err == nil || !strings.Contains(err.Error(), "nil runtime") {
		t.Fatalf("nil runtime: %v", err)
	}
	if err := Scan(&Runtime{}); err == nil || !strings.Contains(err.Error(), "nil config") {
		t.Fatalf("nil config: %v", err)
	}
	rt := &Runtime{Config: loadKratosConfig(t, "app: {}")}
	if err := Scan(rt, nil); err == nil || !strings.Contains(err.Error(), "target[0]: nil") {
		t.Fatalf("nil target: %v", err)
	}
	var typedNil *corsv1.CORS
	if err := Scan(rt, typedNil); err == nil || !strings.Contains(err.Error(), "typed nil") {
		t.Fatalf("typed nil target: %v", err)
	}
}

func TestScanWholeConfigDoesNotReadSameNamedSection(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
server:
  http:
    listen:
      addr: ":8080"
`)}
	cfg := &corev1.Bootstrap{}
	if err := Scan(rt, cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.GetServer().GetHttp().GetListen().GetNetwork(); got != "tcp" {
		t.Fatalf("whole config default network = %q", got)
	}
}

func TestScanUsesAcronymAndCompositeMessageKeys(t *testing.T) {
	if got := sectionKey(&apidocsv1.APIDocs{}); got != "api_docs" {
		t.Fatalf("sectionKey(APIDocs) = %q, want api_docs", got)
	}
	rt := &Runtime{Config: loadKratosConfig(t, `
redis:
  addr: "127.0.0.1:6379"
cors:
  enable: true
`)}
	redis, cors := &redispb.Redis{}, &corsv1.CORS{}
	if err := Scan(rt, redis, cors); err != nil {
		t.Fatal(err)
	}
	if redis.GetAddr() != "127.0.0.1:6379" || redis.GetDialTimeout().AsDuration() != 5*time.Second || !cors.GetEnable() {
		t.Fatalf("automatic section keys did not load or apply: redis=%v cors=%v", redis, cors)
	}
}

func TestScanMissingSectionSkipsApply(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "unrelated: true")}
	cfg := &corsv1.CORS{Enable: true}
	if err := Scan(rt, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.MaxAge != nil {
		t.Fatalf("missing section unexpectedly applied max age: %v", cfg.MaxAge)
	}
}

func TestScanPresentSectionAppliesAndReportsErrors(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "cors: {enable: true}\n")}
	cfg := &corsv1.CORS{}
	if err := Scan(rt, cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.GetEnable() || cfg.GetMaxAge() == nil || cfg.GetMaxAge().AsDuration() != 24*time.Hour {
		t.Fatalf("present CORS section not applied: %v", cfg)
	}

	invalidType := &Runtime{Config: loadKratosConfig(t, "cors: 42\n")}
	if err := Scan(invalidType, &corsv1.CORS{}); err == nil || !strings.Contains(err.Error(), `section "cors"`) {
		t.Fatalf("invalid section type: %v", err)
	}

	invalidRule := &Runtime{Config: loadKratosConfig(t, "obs: {log: {backends: [{file: {}}]}}\n")}
	if err := Scan(invalidRule, &corev1.Bootstrap{}); err == nil || !strings.Contains(err.Error(), "path") {
		t.Fatalf("invalid nested file backend: %v", err)
	}
}

package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

// loadKratosConfig writes the supplied yaml to a tempfile and returns a loaded
// kratos config. The cleanup is registered via t.Cleanup.
func loadKratosConfig(t *testing.T, yaml string) kconfig.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bootstrap.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write tmp yaml: %v", err)
	}
	cfg := kconfig.New(
		kconfig.WithSource(file.NewSource(path)),
		kconfig.WithResolveActualTypes(true),
	)
	if err := cfg.Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	t.Cleanup(func() { _ = cfg.Close() })
	return cfg
}

type stubSection struct {
	Name  string `json:"name"`
	Count int    `json:"count"`

	key            string
	optional       bool
	applyCalls     int
	validateCalls  int
	validateResult error
}

func (s *stubSection) SectionKey() string    { return s.key }
func (s *stubSection) SectionOptional() bool { return s.optional }
func (s *stubSection) ApplyDefaults() {
	s.applyCalls++
	if s.Name == "" {
		s.Name = "filled-default"
	}
}
func (s *stubSection) CheckRequired() error { s.validateCalls++; return s.validateResult }
func (s *stubSection) ApplyConf() error {
	if err := s.CheckRequired(); err != nil {
		return err
	}
	s.ApplyDefaults()
	return nil
}

// minimalSection has SectionKey only (no optional / defaulter / validator).
type minimalSection struct {
	Greeting string `json:"greeting"`
}

func (*minimalSection) SectionKey() string { return "hello" }

type wholeConfig struct {
	Seed struct {
		AdminName string `json:"admin_name"`
	} `json:"seed"`
	applyCalls int
	applyErr   error
}

func (c *wholeConfig) ApplyConf() error {
	c.applyCalls++
	return c.applyErr
}

func TestScan_NilRuntime(t *testing.T) {
	if err := Scan(nil); err == nil || err.Error() != "bootstrap: scan: nil runtime" {
		t.Fatalf("Scan(nil) error = %v, want nil runtime", err)
	}
}

func TestScan_NilConfig(t *testing.T) {
	if err := Scan(&Runtime{}); err == nil || err.Error() != "bootstrap: scan: nil config" {
		t.Fatalf("Scan(nil config) error = %v, want nil config", err)
	}
}

func TestScan_NilTarget(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "")}
	if err := Scan(rt, nil); err == nil || err.Error() != "bootstrap: scan target[0]: nil" {
		t.Fatalf("Scan(nil target) error = %v, want nil target", err)
	}
}

func TestScan_TypedNilTarget(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "")}
	var s *stubSection
	if err := Scan(rt, s); err == nil || err.Error() != "bootstrap: scan target[0]: typed nil *bootstrap.stubSection" {
		t.Fatalf("Scan(typed nil) error = %v, want typed nil", err)
	}
}

func TestScan_WholeConfig(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
seed:
  admin_name: "root-admin"
`)}
	cfg := &wholeConfig{}
	if err := Scan(rt, cfg); err != nil {
		t.Fatalf("Scan whole config error = %v", err)
	}
	if cfg.Seed.AdminName != "root-admin" {
		t.Fatalf("admin_name = %q, want root-admin", cfg.Seed.AdminName)
	}
	if cfg.applyCalls != 1 {
		t.Fatalf("ApplyConf calls = %d, want 1", cfg.applyCalls)
	}
}

func TestScan_WholeConfigApplyError(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `seed: { admin_name: "root" }`)}
	want := errors.New("apply failed")
	cfg := &wholeConfig{applyErr: want}
	err := Scan(rt, cfg)
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "bootstrap: apply target[0] config") {
		t.Fatalf("error = %v, want config apply wrap", err)
	}
}

func TestScan_EmptySectionKey(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "")}
	s := &stubSection{key: ""}
	if err := Scan(rt, s); err == nil || err.Error() != "bootstrap: scan target[0]: empty section key" {
		t.Fatalf("Scan empty key error = %v, want empty key", err)
	}
}

func TestScan_PresentSection(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
biz:
  name: "found-name"
  count: 42
`)}
	s := &stubSection{key: "biz"}
	if err := Scan(rt, s); err != nil {
		t.Fatalf("Scan section error = %v", err)
	}
	if s.Name != "found-name" {
		t.Fatalf("Name = %q, want %q", s.Name, "found-name")
	}
	if s.Count != 42 {
		t.Fatalf("Count = %d, want 42", s.Count)
	}
	if s.applyCalls != 1 {
		t.Fatalf("ApplyDefaults called %d times, want 1", s.applyCalls)
	}
	if s.validateCalls != 1 {
		t.Fatalf("CheckRequired called %d times, want 1", s.validateCalls)
	}
}

func TestScan_OptionalMissingSkipsApplyConf(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `other: "value"`)}
	s := &stubSection{key: "biz", optional: true}
	if err := Scan(rt, s); err != nil {
		t.Fatalf("optional missing should not error, got: %v", err)
	}
	if s.applyCalls != 0 {
		t.Fatalf("ApplyConf should be skipped when section missing, got %d calls", s.applyCalls)
	}
	if s.Name != "" {
		t.Fatalf("Name = %q, want empty because ApplyConf was skipped", s.Name)
	}
}

func TestScan_RequiredMissing(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `other: "value"`)}
	s := &stubSection{key: "biz", optional: false}
	err := Scan(rt, s)
	if err == nil || !strings.Contains(err.Error(), `bootstrap: scan target[0] section "biz"`) {
		t.Fatalf("required missing error = %v, want section scan wrap", err)
	}
}

func TestScan_RequiredCheckerFailFast(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
a: { name: "ok" }
b: { name: "ok" }
`)}
	first := &stubSection{key: "a", validateResult: errors.New("first failed")}
	second := &stubSection{key: "b"}
	err := Scan(rt, first, second)
	if err == nil || err.Error() != `bootstrap: apply target[0] section "a": first failed` {
		t.Fatalf("error = %v, want section apply wrap", err)
	}
	if first.validateCalls != 1 {
		t.Fatalf("first.CheckRequired calls = %d, want 1", first.validateCalls)
	}
	if second.validateCalls != 0 {
		t.Fatalf("second should not be processed, got %d calls", second.validateCalls)
	}
}

func TestScan_DottedKey(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
infra:
  broker:
    name: "inner-broker"
    count: 7
`)}
	s := &stubSection{key: "infra.broker"}
	if err := Scan(rt, s); err != nil {
		t.Fatalf("dotted key scan err: %v", err)
	}
	if s.Name != "inner-broker" || s.Count != 7 {
		t.Fatalf("dotted scan result = (%q, %d), want (inner-broker, 7)", s.Name, s.Count)
	}
}

func TestScan_MinimalNoOptionsOrCheckRequired(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
hello:
  greeting: "hi"
`)}
	m := &minimalSection{}
	if err := Scan(rt, m); err != nil {
		t.Fatalf("minimal section err: %v", err)
	}
	if m.Greeting != "hi" {
		t.Fatalf("Greeting = %q, want hi", m.Greeting)
	}
}

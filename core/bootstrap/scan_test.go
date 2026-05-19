package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
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
func (s *stubSection) ApplyDefaults()        { s.applyCalls++; if s.Name == "" { s.Name = "filled-default" } }
func (s *stubSection) CheckRequired() error  { s.validateCalls++; return s.validateResult }
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

func TestScanSections_NilRuntime(t *testing.T) {
	if err := ScanSections(nil); err == nil {
		t.Fatal("ScanSections(nil) returned nil, want error")
	}
}

func TestScanSections_EmptyKey(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "")}
	s := &stubSection{key: ""}
	if err := ScanSections(rt, s); err == nil {
		t.Fatal("ScanSections with empty key returned nil, want error")
	}
}

func TestScanSections_PresentSection(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
biz:
  name: "found-name"
  count: 42
`)}
	s := &stubSection{key: "biz"}
	if err := ScanSections(rt, s); err != nil {
		t.Fatalf("ScanSections returned err: %v", err)
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

func TestScanSections_OptionalMissing(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `other: "value"`)}
	s := &stubSection{key: "biz", optional: true}
	if err := ScanSections(rt, s); err != nil {
		t.Fatalf("optional missing should not error, got: %v", err)
	}
	if s.applyCalls != 1 {
		t.Fatalf("ApplyDefaults should run even when section missing, got %d calls", s.applyCalls)
	}
	if s.Name != "filled-default" {
		t.Fatalf("Name = %q, want %q (default)", s.Name, "filled-default")
	}
}

func TestScanSections_RequiredMissing(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `other: "value"`)}
	s := &stubSection{key: "biz", optional: false}
	if err := ScanSections(rt, s); err == nil {
		t.Fatal("required missing should return error")
	}
}

func TestScanSections_RequiredCheckerFailFast(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
a: { name: "ok" }
b: { name: "ok" }
`)}
	first := &stubSection{key: "a", validateResult: errors.New("first failed")}
	second := &stubSection{key: "b"}
	err := ScanSections(rt, first, second)
	if err == nil || err.Error() != `section "a": first failed` {
		t.Fatalf("error = %v, want section %q wrap", err, "a")
	}
	if first.validateCalls != 1 {
		t.Fatalf("first.CheckRequired calls = %d, want 1", first.validateCalls)
	}
	if second.validateCalls != 0 {
		t.Fatalf("second should not be processed, got %d calls", second.validateCalls)
	}
}

func TestScanSections_DottedKey(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
infra:
  broker:
    name: "inner-broker"
    count: 7
`)}
	s := &stubSection{key: "infra.broker"}
	if err := ScanSections(rt, s); err != nil {
		t.Fatalf("dotted key scan err: %v", err)
	}
	if s.Name != "inner-broker" || s.Count != 7 {
		t.Fatalf("dotted scan result = (%q, %d), want (inner-broker, 7)", s.Name, s.Count)
	}
}

func TestScanSections_MinimalNoOptionsOrCheckRequired(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, `
hello:
  greeting: "hi"
`)}
	m := &minimalSection{}
	if err := ScanSections(rt, m); err != nil {
		t.Fatalf("minimal section err: %v", err)
	}
	if m.Greeting != "hi" {
		t.Fatalf("Greeting = %q, want hi", m.Greeting)
	}
}

func TestScanSections_NilSectionInList(t *testing.T) {
	rt := &Runtime{Config: loadKratosConfig(t, "")}
	if err := ScanSections(rt, (*stubSection)(nil)); err == nil {
		t.Fatal("nil section in list should error")
	}
}

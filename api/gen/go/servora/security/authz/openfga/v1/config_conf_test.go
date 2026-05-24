package openfgaconfpb

import "testing"

func TestConfigCheckRequired(t *testing.T) {
	cfg := &Config{StoreId: "store"}
	if err := cfg.CheckRequired(); err == nil {
		t.Fatal("expected missing api_url error")
	}
	cfg = &Config{ApiUrl: "http://localhost:8080"}
	if err := cfg.CheckRequired(); err == nil {
		t.Fatal("expected missing store_id error")
	}
}

func TestConfigOptionalFields(t *testing.T) {
	cfg := &Config{
		ApiUrl:  "http://localhost:8080",
		StoreId: "store",
	}
	if err := cfg.ApplyConf(); err != nil {
		t.Fatalf("ApplyConf() error = %v", err)
	}
}

func TestConfigSection(t *testing.T) {
	cfg := &Config{}
	if cfg.SectionKey() != "authz.openfga" {
		t.Fatalf("SectionKey() = %q, want authz.openfga", cfg.SectionKey())
	}
	if !cfg.SectionOptional() {
		t.Fatal("SectionOptional() = false, want true")
	}
}

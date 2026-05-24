package oidcconfpb

import "testing"

func TestConfigApplyDefaults(t *testing.T) {
	cfg := &Config{
		CryptoKey:    "secret",
		LoginBaseUrl: "https://login.example.test",
	}
	if err := cfg.ApplyConf(); err != nil {
		t.Fatalf("ApplyConf() error = %v", err)
	}
	if cfg.GrantTypeRefreshToken {
		t.Fatal("GrantTypeRefreshToken = true, want false")
	}
	if cfg.DefaultLogoutRedirectUri != "/" {
		t.Fatalf("DefaultLogoutRedirectUri = %q, want /", cfg.DefaultLogoutRedirectUri)
	}
}

func TestConfigCheckRequired(t *testing.T) {
	cfg := &Config{LoginBaseUrl: "https://login.example.test"}
	if err := cfg.CheckRequired(); err == nil {
		t.Fatal("expected missing crypto_key error")
	}
	cfg = &Config{CryptoKey: "secret"}
	if err := cfg.CheckRequired(); err == nil {
		t.Fatal("expected missing login_base_url error")
	}
}

func TestConfigSection(t *testing.T) {
	cfg := &Config{}
	if cfg.SectionKey() != "authn.oidc" {
		t.Fatalf("SectionKey() = %q, want authn.oidc", cfg.SectionKey())
	}
	if !cfg.SectionOptional() {
		t.Fatal("SectionOptional() = false, want true")
	}
}

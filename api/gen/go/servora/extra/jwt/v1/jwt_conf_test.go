package jwtv1

import "testing"

func TestJwtApplyDefaults(t *testing.T) {
	cfg := &Jwt{}
	cfg.ApplyDefaults()
	if cfg.AccessExpire != 3600 {
		t.Fatalf("AccessExpire = %d, want 3600", cfg.AccessExpire)
	}
	if cfg.RefreshExpire != 604800 {
		t.Fatalf("RefreshExpire = %d, want 604800", cfg.RefreshExpire)
	}
}

func TestJwtSecurityFieldsHaveNoDefaults(t *testing.T) {
	cfg := &Jwt{}
	cfg.ApplyDefaults()
	if cfg.Issuer != "" {
		t.Fatalf("Issuer = %q, want empty", cfg.Issuer)
	}
	if cfg.Audience != "" {
		t.Fatalf("Audience = %q, want empty", cfg.Audience)
	}
	if cfg.PrivateKeyPath != "" {
		t.Fatalf("PrivateKeyPath = %q, want empty", cfg.PrivateKeyPath)
	}
	if cfg.PrivateKeyPem != "" {
		t.Fatalf("PrivateKeyPem = %q, want empty", cfg.PrivateKeyPem)
	}
}

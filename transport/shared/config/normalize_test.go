package config

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNormalizeDuration(t *testing.T) {
	if got := NormalizeDuration(nil, 5*time.Second); got != 5*time.Second {
		t.Fatalf("duration = %s, want %s", got, 5*time.Second)
	}
	if got := NormalizeDuration(durationpb.New(0), 5*time.Second); got != 5*time.Second {
		t.Fatalf("duration = %s, want %s", got, 5*time.Second)
	}
	if got := NormalizeDuration(durationpb.New(3*time.Second), 5*time.Second); got != 3*time.Second {
		t.Fatalf("duration = %s, want %s", got, 3*time.Second)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	if got := NormalizeEndpoint("", "discovery:///svc"); got != "discovery:///svc" {
		t.Fatalf("endpoint = %q, want %q", got, "discovery:///svc")
	}
	if got := NormalizeEndpoint("  dns:///svc.internal:9000  ", "discovery:///svc"); got != "dns:///svc.internal:9000" {
		t.Fatalf("endpoint = %q, want %q", got, "dns:///svc.internal:9000")
	}
}

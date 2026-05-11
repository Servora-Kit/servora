package authn

import (
	"context"
	"testing"
)

func TestWithAuthType_Roundtrip(t *testing.T) {
	ctx := context.Background()
	ctx = WithAuthType(ctx, "jwt")

	got, ok := AuthTypeFrom(ctx)
	if !ok {
		t.Fatal("AuthTypeFrom returned ok=false, want true")
	}
	if got != "jwt" {
		t.Errorf("AuthTypeFrom = %q, want jwt", got)
	}
}

func TestAuthTypeFrom_Missing(t *testing.T) {
	ctx := context.Background()
	got, ok := AuthTypeFrom(ctx)
	if ok {
		t.Error("AuthTypeFrom returned ok=true on empty ctx, want false")
	}
	if got != "" {
		t.Errorf("AuthTypeFrom = %q, want empty string", got)
	}
}

func TestWithAuthType_Overwrites(t *testing.T) {
	ctx := context.Background()
	ctx = WithAuthType(ctx, "apikey")
	ctx = WithAuthType(ctx, "mtls")

	got, ok := AuthTypeFrom(ctx)
	if !ok {
		t.Fatal("AuthTypeFrom returned ok=false after overwrite")
	}
	if got != "mtls" {
		t.Errorf("AuthTypeFrom = %q, want mtls (last-wins)", got)
	}
}

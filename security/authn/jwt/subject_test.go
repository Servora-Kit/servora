package jwt

import (
	"context"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"
)

func TestSubjectFrom(t *testing.T) {
	cases := []struct {
		name    string
		ctx     context.Context
		wantSub string
		wantOK  bool
	}{
		{
			name:    "no-claims-in-ctx",
			ctx:     context.Background(),
			wantSub: "",
			wantOK:  false,
		},
		{
			name:    "claims-present-with-sub",
			ctx:     WithClaims(context.Background(), gojwt.MapClaims{"sub": "user-123"}),
			wantSub: "user-123",
			wantOK:  true,
		},
		{
			name:    "claims-present-without-sub",
			ctx:     WithClaims(context.Background(), gojwt.MapClaims{"name": "Alice"}),
			wantSub: "",
			wantOK:  false,
		},
		{
			name:    "sub-is-not-string",
			ctx:     WithClaims(context.Background(), gojwt.MapClaims{"sub": 12345}),
			wantSub: "",
			wantOK:  false,
		},
		{
			name:    "sub-is-empty-string",
			ctx:     WithClaims(context.Background(), gojwt.MapClaims{"sub": ""}),
			wantSub: "",
			wantOK:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SubjectFrom(tc.ctx)
			if ok != tc.wantOK {
				t.Errorf("SubjectFrom() ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.wantSub {
				t.Errorf("SubjectFrom() = %q, want %q", got, tc.wantSub)
			}
		})
	}
}

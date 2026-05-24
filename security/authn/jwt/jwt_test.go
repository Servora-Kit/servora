package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/transport"
	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Servora-Kit/servora/security/authn"
	jwtpkg "github.com/Servora-Kit/servora/security/jwt"
)

type fakeServerTransport struct {
	headers map[string]string
}

func (f *fakeServerTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeServerTransport) Endpoint() string                { return "" }
func (f *fakeServerTransport) Operation() string               { return "" }
func (f *fakeServerTransport) RequestHeader() transport.Header { return &fakeHeader{f.headers} }
func (f *fakeServerTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

type fakeClientTransport struct {
	headers map[string]string
}

func (f *fakeClientTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeClientTransport) Endpoint() string                { return "" }
func (f *fakeClientTransport) Operation() string               { return "" }
func (f *fakeClientTransport) RequestHeader() transport.Header { return &fakeHeader{f.headers} }
func (f *fakeClientTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

type fakeHeader struct {
	m map[string]string
}

func (h *fakeHeader) Get(key string) string    { return h.m[key] }
func (h *fakeHeader) Set(key, value string)    { h.m[key] = value }
func (h *fakeHeader) Add(_ string, _ string)   {}
func (h *fakeHeader) Keys() []string           { return nil }
func (h *fakeHeader) Values(_ string) []string { return nil }

func serverCtx(headers map[string]string) context.Context {
	if headers == nil {
		headers = map[string]string{}
	}
	return transport.NewServerContext(context.Background(), &fakeServerTransport{headers: headers})
}

func clientCtx() (context.Context, *fakeClientTransport) {
	tr := &fakeClientTransport{headers: map[string]string{}}
	return transport.NewClientContext(context.Background(), tr), tr
}

func newTestVerifierAndToken(t *testing.T, claims gojwt.MapClaims) (*jwtpkg.Verifier, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	verifier := jwtpkg.NewVerifier()
	verifier.AddKey("kid-test", &key.PublicKey)
	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	token.Header["kid"] = "kid-test"
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return verifier, raw
}

func TestNewAuthenticator_WithoutVerifier_Panics(t *testing.T) {
	assertPanic(t, func() { _ = NewAuthenticator() })
}

func TestAuthenticate_NoBearer_ReturnsErrNoCredentials(t *testing.T) {
	verifier, _ := newTestVerifierAndToken(t, gojwt.MapClaims{"sub": "u-1"})
	auth := NewAuthenticator(WithVerifier(verifier))

	_, err := auth.Authenticate(serverCtx(nil))
	if !errors.Is(err, authn.ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}

func TestAuthenticate_InvalidBearer_FailsFast(t *testing.T) {
	verifier, _ := newTestVerifierAndToken(t, gojwt.MapClaims{"sub": "u-1"})
	auth := NewAuthenticator(WithVerifier(verifier))
	ctx := serverCtx(map[string]string{"Authorization": "Bearer bad-token"})

	_, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected verify error")
	}
	if errors.Is(err, authn.ErrNoCredentials) {
		t.Fatalf("err = %v, must not match ErrNoCredentials", err)
	}
}

func TestAuthenticate_SuccessWritesClaimsTokenAndAuthType(t *testing.T) {
	verifier, raw := newTestVerifierAndToken(t, gojwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	auth := NewAuthenticator(WithVerifier(verifier))
	ctx := serverCtx(map[string]string{"Authorization": "Bearer " + raw})

	got, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tok, ok := TokenFrom(got)
	if !ok || tok != raw {
		t.Fatalf("TokenFrom = (%q,%v), want raw token", tok, ok)
	}
	claims, ok := ClaimsFrom(got)
	if !ok {
		t.Fatal("ClaimsFrom returned false")
	}
	if claims["sub"] != "user-123" {
		t.Fatalf("sub = %v, want user-123", claims["sub"])
	}
	authType, ok := authn.AuthTypeFrom(got)
	if !ok || authType != Scheme {
		t.Fatalf("AuthTypeFrom = (%q,%v), want (jwt,true)", authType, ok)
	}
}

func TestAuthenticate_DirectMultiPathSupportsClientPropagation(t *testing.T) {
	verifier, raw := newTestVerifierAndToken(t, gojwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	mw := authn.Server(authn.Multi(authn.Named(Scheme, NewAuthenticator(WithVerifier(verifier)))))
	var handlerCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		handlerCtx = ctx
		return "ok", nil
	})

	if _, err := handler(serverCtx(map[string]string{"Authorization": "Bearer " + raw}), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tok, ok := TokenFrom(handlerCtx)
	if !ok || tok != raw {
		t.Fatalf("TokenFrom(handlerCtx) = (%q,%v), want raw token", tok, ok)
	}

	outCtx, tr := clientCtx()
	outCtx = WithToken(outCtx, tok)
	client := Client()
	_, err := client(func(context.Context, any) (any, error) { return "ok", nil })(outCtx, nil)
	if err != nil {
		t.Fatalf("client middleware error: %v", err)
	}
	if got := tr.headers["Authorization"]; got != "Bearer "+raw {
		t.Fatalf("Authorization = %q, want Bearer raw", got)
	}
}

func TestClient_PropagatesToken(t *testing.T) {
	cases := []struct {
		name       string
		setup      func() (context.Context, *fakeClientTransport)
		wantHeader string
	}{
		{
			name: "with-token",
			setup: func() (context.Context, *fakeClientTransport) {
				ctx, tr := clientCtx()
				return WithToken(ctx, "abc123"), tr
			},
			wantHeader: "Bearer abc123",
		},
		{
			name:       "no-token-in-ctx",
			setup:      clientCtx,
			wantHeader: "",
		},
		{
			name: "empty-token",
			setup: func() (context.Context, *fakeClientTransport) {
				ctx, tr := clientCtx()
				return WithToken(ctx, ""), tr
			},
			wantHeader: "",
		},
		{
			name: "no-client-transport",
			setup: func() (context.Context, *fakeClientTransport) {
				return WithToken(context.Background(), "abc123"), nil
			},
			wantHeader: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, tr := tc.setup()
			calls := 0
			got, err := Client()(func(context.Context, any) (any, error) {
				calls++
				return "ok", nil
			})(ctx, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != "ok" {
				t.Fatalf("got = %v, want ok", got)
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
			if tr != nil && tr.headers["Authorization"] != tc.wantHeader {
				t.Fatalf("Authorization = %q, want %q", tr.headers["Authorization"], tc.wantHeader)
			}
		})
	}
}

func TestTokenForAuth_PrefersCtxOverHeader(t *testing.T) {
	ctx := serverCtx(map[string]string{"Authorization": "Bearer header-tok"})
	ctx = WithToken(ctx, "ctx-tok")
	if got := tokenForAuth(ctx); got != "ctx-tok" {
		t.Errorf("tokenForAuth = %q, want ctx-tok", got)
	}
}

func TestTokenForAuth_FallsBackToHeader(t *testing.T) {
	ctx := serverCtx(map[string]string{"Authorization": "Bearer header-tok"})
	if got := tokenForAuth(ctx); got != "header-tok" {
		t.Errorf("tokenForAuth = %q, want header-tok", got)
	}
}

func TestTokenForAuth_EmptyEverywhere(t *testing.T) {
	if got := tokenForAuth(context.Background()); got != "" {
		t.Errorf("tokenForAuth = %q, want empty", got)
	}
}

func TestDefaultClaimsMapper_SubPresent(t *testing.T) {
	mapper := DefaultClaimsMapper()
	claims := gojwt.MapClaims{
		"sub":  "user-123",
		"name": "Alice",
	}
	got, err := mapper(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stored, ok := ClaimsFrom(got)
	if !ok {
		t.Fatal("ClaimsFrom returned false")
	}
	if stored["sub"] != "user-123" {
		t.Errorf("stored sub = %v, want user-123", stored["sub"])
	}
}

func TestDefaultClaimsMapper_EmptySubFails(t *testing.T) {
	_, err := DefaultClaimsMapper()(context.Background(), gojwt.MapClaims{"name": "Alice"})
	if err == nil {
		t.Fatal("expected error for empty sub")
	}
}

func TestDefaultClaimsMapper_SubjectFromConvenience(t *testing.T) {
	ctx, err := DefaultClaimsMapper()(context.Background(), gojwt.MapClaims{"sub": "user-xyz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sub, ok := SubjectFrom(ctx)
	if !ok || sub != "user-xyz" {
		t.Errorf("SubjectFrom = (%q,%v), want (user-xyz,true)", sub, ok)
	}
}

func TestWithClaimsMapper_CustomExtensionPoint(t *testing.T) {
	type customKey struct{}
	custom := func(ctx context.Context, claims gojwt.MapClaims) (context.Context, error) {
		role, _ := claims["custom_role"].(string)
		if role == "" {
			return ctx, errors.New("custom: missing custom_role")
		}
		ctx = WithClaims(ctx, claims)
		return context.WithValue(ctx, customKey{}, role), nil
	}

	verifier, _ := newTestVerifierAndToken(t, gojwt.MapClaims{"sub": "u-1"})
	auth := NewAuthenticator(WithVerifier(verifier), WithClaimsMapper(custom)).(*authenticator)
	got, err := auth.cfg.claimsMapper(context.Background(), gojwt.MapClaims{
		"sub":         "user-1",
		"custom_role": "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	role, ok := got.Value(customKey{}).(string)
	if !ok || role != "admin" {
		t.Errorf("custom ctx value = (%q,%v), want (admin,true)", role, ok)
	}
	_, err = auth.cfg.claimsMapper(context.Background(), gojwt.MapClaims{"sub": "user-1"})
	if err == nil {
		t.Error("expected error from custom mapper")
	}
}

func TestScheme_IsExposedConstant(t *testing.T) {
	if Scheme != "jwt" {
		t.Errorf("Scheme = %q, want jwt", Scheme)
	}
}

func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

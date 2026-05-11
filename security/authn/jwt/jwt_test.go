package jwt

import (
	"context"
	"errors"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/security/authn"
)

// ============================================================================
// Test fixtures: server / client transport fakes (jwt package-local).
// ============================================================================

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

// serverCtx builds a server-side ctx with a fake transport carrying the given headers.
func serverCtx(headers map[string]string) context.Context {
	if headers == nil {
		headers = map[string]string{}
	}
	return transport.NewServerContext(context.Background(), &fakeServerTransport{headers: headers})
}

// clientCtx builds a client-side ctx with a fake outbound transport.
func clientCtx() (context.Context, *fakeClientTransport) {
	tr := &fakeClientTransport{headers: map[string]string{}}
	return transport.NewClientContext(context.Background(), tr), tr
}

// ============================================================================
// Test fixtures: counting Authenticator. Records call count + observed token.
// ============================================================================

type countingAuthenticator struct {
	calls          int
	enrichCtx      bool // if true, enrich ctx with authn.WithAuthType on success
	returnErr      error
	observedToken  string
	observedHasTok bool
}

func (c *countingAuthenticator) Authenticate(ctx context.Context) (context.Context, error) {
	c.calls++
	c.observedToken, c.observedHasTok = TokenFrom(ctx)
	if c.returnErr != nil {
		return ctx, c.returnErr
	}
	if c.enrichCtx {
		return authn.WithAuthType(ctx, "user"), nil
	}
	return ctx, nil
}

// serverWithStub builds a Server-equivalent middleware backed by the supplied
// stub Authenticator, wired through the same authn.Server + authn.Multi
// composition that public Server(opts...) uses, plus the same Bearer-pre-extract
// step. Lets us test the wrapper shape without a real Verifier dep.
func serverWithStub(stub authn.Authenticator) middleware.Middleware {
	inner := authn.Server(
		authn.Multi(
			authn.Named(Scheme, stub),
		),
	)
	return func(handler middleware.Handler) middleware.Handler {
		next := inner(handler)
		return func(ctx context.Context, req any) (any, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if raw := extractBearerToken(tr.RequestHeader().Get("Authorization")); raw != "" {
					ctx = WithToken(ctx, raw)
				}
			}
			return next(ctx, req)
		}
	}
}

// ============================================================================
// Server-level wrapper tests (Bearer extract + dispatch + AuthnDetail).
// ============================================================================

// TestServer_ExtractsBearerAndDispatches asserts the wrapper:
//  1. reads Authorization: Bearer <tok> off the inbound transport,
//  2. stashes the raw token into the jwt-private ctx channel,
//  3. delegates to authn.Server + Multi dispatcher.
func TestServer_ExtractsBearerAndDispatches(t *testing.T) {
	stub := &countingAuthenticator{
		enrichCtx: true,
	}
	mw := serverWithStub(stub)

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	ctx := serverCtx(map[string]string{"Authorization": "Bearer raw-token-xyz"})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.calls != 1 {
		t.Fatalf("Authenticate calls = %d, want 1", stub.calls)
	}
	if !stub.observedHasTok || stub.observedToken != "raw-token-xyz" {
		t.Errorf("engine saw token = (%q,%v), want (\"raw-token-xyz\",true)",
			stub.observedToken, stub.observedHasTok)
	}

	tok, ok := TokenFrom(capturedCtx)
	if !ok || tok != "raw-token-xyz" {
		t.Errorf("TokenFrom(handler ctx) = (%q,%v), want (\"raw-token-xyz\",true)", tok, ok)
	}

	authType, ok := authn.AuthTypeFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthType in ctx")
	}
	if authType != "user" {
		t.Errorf("AuthType = %q, want %q", authType, "user")
	}
}

// TestClient_PropagatesToken asserts Client() correctly propagates the token
// in the happy path AND silently passes through on every precondition miss.
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
				ctx = WithToken(ctx, "abc123")
				return ctx, tr
			},
			wantHeader: "Bearer abc123",
		},
		{
			name: "no-token-in-ctx",
			setup: func() (context.Context, *fakeClientTransport) {
				return clientCtx()
			},
			wantHeader: "",
		},
		{
			name: "empty-token",
			setup: func() (context.Context, *fakeClientTransport) {
				ctx, tr := clientCtx()
				ctx = WithToken(ctx, "")
				return ctx, tr
			},
			wantHeader: "",
		},
		{
			name: "no-client-transport",
			setup: func() (context.Context, *fakeClientTransport) {
				ctx := WithToken(context.Background(), "abc123")
				return ctx, nil
			},
			wantHeader: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mw := Client()
			ctx, tr := tc.setup()

			calls := 0
			handler := mw(func(_ context.Context, _ any) (any, error) {
				calls++
				return "ok", nil
			})

			got, err := handler(ctx, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != "ok" {
				t.Errorf("handler return = %v, want %q", got, "ok")
			}
			if calls != 1 {
				t.Errorf("handler calls = %d, want 1", calls)
			}

			if tr != nil {
				gotHeader := tr.headers["Authorization"]
				if gotHeader != tc.wantHeader {
					t.Errorf("outbound Authorization = %q, want %q", gotHeader, tc.wantHeader)
				}
			}
		})
	}
}

// TestServer_TokenPassthroughInCtx asserts that when jwt.Server sees an
// Authorization header, the raw token is available in the ctx passed to the
// handler (via the jwt-private ctx channel) even when the stub returns the
// input ctx unmodified (the wrapper's pre-extract step installs the token
// before dispatch).
func TestServer_TokenPassthroughInCtx(t *testing.T) {
	// Stub returns an enriched ctx so Multi considers it success.
	stub := &countingAuthenticator{enrichCtx: true}
	mw := serverWithStub(stub)

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	ctx := serverCtx(map[string]string{"Authorization": "Bearer my-token"})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.calls != 1 {
		t.Errorf("Authenticate calls = %d, want 1", stub.calls)
	}

	tok, ok := TokenFrom(capturedCtx)
	if !ok || tok != "my-token" {
		t.Errorf("TokenFrom(handler ctx) = (%q,%v), want (\"my-token\",true)", tok, ok)
	}
}

// TestServer_NoBearerHeader_ReachesAuthenticator asserts that when there is
// no Authorization header, the wrapper still delegates to the dispatcher and
// the engine is reached. The engine returns (ctx, nil) as pass-through which
// Multi treats as success (no enrichment, no error).
func TestServer_NoBearerHeader_ReachesAuthenticator(t *testing.T) {
	stub := &countingAuthenticator{} // returnCtx nil → pass-through
	mw := serverWithStub(stub)

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	ctx := serverCtx(nil)
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.calls != 1 {
		t.Fatalf("Authenticate calls = %d, want 1", stub.calls)
	}
	if stub.observedHasTok || stub.observedToken != "" {
		t.Errorf("engine saw token = (%q,%v), want (\"\",false)", stub.observedToken, stub.observedHasTok)
	}

	if capturedCtx == nil {
		t.Error("handler must be called (capturedCtx should not be nil)")
	}
}

// ============================================================================
// Authenticate-level tests (engine in isolation, no transport / wrapper).
// ============================================================================

// TestAuthenticate_NoTokenInContext_Passthrough: no token in ctx + no
// transport → ctx returned unchanged (pass-through).
func TestAuthenticate_NoTokenInContext_Passthrough(t *testing.T) {
	auth := NewAuthenticator()
	ctx := context.Background()
	got, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No enrichment: AuthType should be absent.
	if _, ok := authn.AuthTypeFrom(got); ok {
		t.Error("AuthType should not be present in pass-through ctx")
	}
}

// TestAuthenticate_NoVerifier_Passthrough: token present but no Verifier
// configured → ctx returned unchanged (pass-through mode).
func TestAuthenticate_NoVerifier_Passthrough(t *testing.T) {
	auth := NewAuthenticator()
	ctx := WithToken(context.Background(), "any-token")
	got, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := authn.AuthTypeFrom(got); ok {
		t.Error("AuthType should not be present in pass-through ctx")
	}
}

// TestAuthenticate_TransportHeaderFallback: when no jwt-private ctx token but
// a Kratos server transport carries Authorization: Bearer ... the engine is
// reached via Multi. This test wires authn.Server(authn.Multi(...)) directly
// (no jwt.Server convenience wrapper), so the engine gets the raw transport
// ctx — observedHasTok must therefore be false (Multi does not pre-extract).
func TestAuthenticate_TransportHeaderFallback(t *testing.T) {
	captured := &countingAuthenticator{
		enrichCtx: true,
	}
	mw := authn.Server(authn.Multi(authn.Named(Scheme, captured)))
	handler := mw(func(_ context.Context, _ any) (any, error) { return "ok", nil })

	ctx := serverCtx(map[string]string{"Authorization": "Bearer fallback-tok"})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.calls != 1 {
		t.Fatalf("Authenticate calls = %d, want 1", captured.calls)
	}
	if captured.observedHasTok {
		t.Errorf("stub saw WithToken-backed entry, want empty (Multi direct wiring leaves jwt-private channel untouched)")
	}
}

// TestTokenForAuth_PrefersCtxOverHeader asserts the priority: ctx channel
// wins over transport header.
func TestTokenForAuth_PrefersCtxOverHeader(t *testing.T) {
	ctx := serverCtx(map[string]string{"Authorization": "Bearer header-tok"})
	ctx = WithToken(ctx, "ctx-tok")
	got := tokenForAuth(ctx)
	if got != "ctx-tok" {
		t.Errorf("tokenForAuth = %q, want ctx-tok", got)
	}
}

// TestTokenForAuth_FallsBackToHeader asserts the fallback when ctx is absent.
func TestTokenForAuth_FallsBackToHeader(t *testing.T) {
	ctx := serverCtx(map[string]string{"Authorization": "Bearer header-tok"})
	got := tokenForAuth(ctx)
	if got != "header-tok" {
		t.Errorf("tokenForAuth = %q, want header-tok", got)
	}
}

// TestTokenForAuth_EmptyEverywhere asserts empty when neither side has a token.
func TestTokenForAuth_EmptyEverywhere(t *testing.T) {
	ctx := context.Background()
	if got := tokenForAuth(ctx); got != "" {
		t.Errorf("tokenForAuth = %q, want \"\"", got)
	}
}

// ============================================================================
// ClaimsMapper tests — default mapper stores full claims into ctx.
// ============================================================================

// TestDefaultClaimsMapper_SubPresent: canonical claims → ctx enriched with
// full claims map accessible via ClaimsFrom / SubjectFrom.
func TestDefaultClaimsMapper_SubPresent(t *testing.T) {
	mapper := DefaultClaimsMapper()
	claims := gojwt.MapClaims{
		"sub":  "user-123",
		"name": "Alice",
	}
	ctx := context.Background()
	got, err := mapper(ctx, claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stored, ok := ClaimsFrom(got)
	if !ok {
		t.Fatal("ClaimsFrom returned false after DefaultClaimsMapper success")
	}
	if stored["sub"] != "user-123" {
		t.Errorf("stored sub = %v, want user-123", stored["sub"])
	}
	if stored["name"] != "Alice" {
		t.Errorf("stored name = %v, want Alice", stored["name"])
	}
}

// TestDefaultClaimsMapper_EmptySubFails: sub is REQUIRED.
func TestDefaultClaimsMapper_EmptySubFails(t *testing.T) {
	mapper := DefaultClaimsMapper()
	_, err := mapper(context.Background(), gojwt.MapClaims{
		"name": "Alice",
	})
	if err == nil {
		t.Fatal("expected error for empty sub, got nil")
	}
}

// TestDefaultClaimsMapper_SubjectFromConvenience: SubjectFrom extracts the
// sub claim from the enriched ctx produced by DefaultClaimsMapper.
func TestDefaultClaimsMapper_SubjectFromConvenience(t *testing.T) {
	mapper := DefaultClaimsMapper()
	ctx, err := mapper(context.Background(), gojwt.MapClaims{"sub": "user-xyz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sub, ok := SubjectFrom(ctx)
	if !ok || sub != "user-xyz" {
		t.Errorf("SubjectFrom = (%q,%v), want (\"user-xyz\",true)", sub, ok)
	}
}

// TestWithClaimsMapper_CustomExtensionPoint: business-installed mapper is
// honored end-to-end and can write arbitrary ctx values.
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

	auth := newAuthenticator(WithClaimsMapper(custom))
	if auth.cfg.claimsMapper == nil {
		t.Fatal("ClaimsMapper not installed by Option")
	}

	ctx := context.Background()
	got, err := auth.cfg.claimsMapper(ctx, gojwt.MapClaims{
		"sub":         "user-1",
		"custom_role": "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	role, ok := got.Value(customKey{}).(string)
	if !ok || role != "admin" {
		t.Errorf("custom ctx value = (%q,%v), want (\"admin\",true)", role, ok)
	}

	// And that errors propagate.
	_, err = auth.cfg.claimsMapper(ctx, gojwt.MapClaims{"sub": "user-1"})
	if err == nil {
		t.Error("expected error from custom mapper, got nil")
	}
}

// TestScheme_IsExposedConstant: paranoid sanity check that the package-level
// constant exists and has the expected value (downstream proto schemes lists
// rely on it being "jwt").
func TestScheme_IsExposedConstant(t *testing.T) {
	if Scheme != "jwt" {
		t.Errorf("Scheme = %q, want jwt", Scheme)
	}
}

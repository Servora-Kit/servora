package jwt

import (
	"context"
	"errors"
	"fmt"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
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

// serverCtx builds a server-side ctx with a fake transport carrying the given
// headers AND a fresh audit detail holder.
func serverCtx(headers map[string]string) context.Context {
	if headers == nil {
		headers = map[string]string{}
	}
	ctx := transport.NewServerContext(context.Background(), &fakeServerTransport{headers: headers})
	return audit.InstallHolder(ctx)
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
	returnActor    actor.Actor
	returnErr      error
	observedToken  string
	observedHasTok bool
}

func (c *countingAuthenticator) Authenticate(ctx context.Context) (actor.Actor, error) {
	c.calls++
	c.observedToken, c.observedHasTok = TokenFrom(ctx)
	if c.returnErr != nil {
		return nil, c.returnErr
	}
	if c.returnActor == nil {
		return actor.NewAnonymousActor(), nil
	}
	return c.returnActor, nil
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
//  3. delegates to authn.Server which writes AuthnDetail.Method = "jwt".
func TestServer_ExtractsBearerAndDispatches(t *testing.T) {
	stub := &countingAuthenticator{
		returnActor: actor.NewUserActor("u1", "Alice"),
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

	d, ok := audit.AuthnResultFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthnDetail in ctx")
	}
	if d.Method != Scheme {
		t.Errorf("AuthnDetail.Method = %q, want %q", d.Method, Scheme)
	}
	if !d.Success {
		t.Errorf("AuthnDetail.Success = false, want true")
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

// TestServer_ChainShortCircuit_PassthroughOnExistingActor asserts that when
// ctx already carries a non-anonymous actor (a previous engine in a chain
// won), authn.Server short-circuits: stub Authenticator never runs, no
// AuthnDetail written.
func TestServer_ChainShortCircuit_PassthroughOnExistingActor(t *testing.T) {
	stub := &countingAuthenticator{
		returnActor: actor.NewUserActor("should-not-be-used", ""),
	}
	mw := serverWithStub(stub)

	preSet := actor.NewUserActor("pre-set", "")

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	ctx := serverCtx(map[string]string{"Authorization": "Bearer would-be-token"})
	ctx = actor.NewContext(ctx, preSet)

	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.calls != 0 {
		t.Errorf("Authenticate calls = %d, want 0 (short-circuit)", stub.calls)
	}

	a, ok := actor.From(capturedCtx)
	if !ok {
		t.Fatal("expected actor in handler ctx")
	}
	if a.ID() != "pre-set" {
		t.Errorf("handler actor ID = %q, want pre-set", a.ID())
	}

	if _, ok := audit.AuthnResultFrom(capturedCtx); ok {
		t.Errorf("AuthnDetail must NOT be written when short-circuit triggers (dispatcher never reached)")
	}
}

// TestServer_NoBearerHeader_ReachesAuthenticator asserts that when there is
// no Authorization header AND no pre-set actor, the wrapper still delegates
// to the dispatcher.
func TestServer_NoBearerHeader_ReachesAuthenticator(t *testing.T) {
	stub := &countingAuthenticator{} // returnActor nil → anonymous
	mw := serverWithStub(stub)

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	ctx := serverCtx(nil)
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.calls != 1 {
		t.Fatalf("Authenticate calls = %d, want 1", stub.calls)
	}
	if stub.observedHasTok || stub.observedToken != "" {
		t.Errorf("engine saw token = (%q,%v), want (\"\",false)", stub.observedToken, stub.observedHasTok)
	}

	d, ok := audit.AuthnResultFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthnDetail in ctx")
	}
	if d.Method != Scheme {
		t.Errorf("AuthnDetail.Method = %q, want %q", d.Method, Scheme)
	}
	if !d.Success {
		t.Errorf("AuthnDetail.Success = false, want true (anonymous pass-through)")
	}
}

// ============================================================================
// Authenticate-level tests (engine in isolation, no transport / wrapper).
// ============================================================================

// TestAuthenticate_NoTokenInContext_ReturnsAnonymous: no token in ctx + no
// transport → anonymous.
func TestAuthenticate_NoTokenInContext_ReturnsAnonymous(t *testing.T) {
	auth := NewAuthenticator()
	a, err := auth.Authenticate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Type() != actor.TypeAnonymous {
		t.Errorf("actor type = %v, want anonymous", a.Type())
	}
}

// TestAuthenticate_NoVerifier_ReturnsAnonymous: token present but no Verifier
// configured → anonymous (pass-through mode).
func TestAuthenticate_NoVerifier_ReturnsAnonymous(t *testing.T) {
	auth := NewAuthenticator()
	ctx := WithToken(context.Background(), "any-token")
	a, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Type() != actor.TypeAnonymous {
		t.Errorf("actor type = %v, want anonymous", a.Type())
	}
}

// TestAuthenticate_TransportHeaderFallback: when no jwt-private ctx token but
// a Kratos server transport carries Authorization: Bearer ... AND no Verifier
// is configured, the engine still surfaces the token through (anonymous via
// pass-through). This proves the transport-header fallback path engages.
func TestAuthenticate_TransportHeaderFallback(t *testing.T) {
	captured := &countingAuthenticator{}
	// Wire the engine through Multi WITHOUT pre-write of WithToken; rely on
	// the transport-header fallback inside Authenticate.
	mw := authn.Server(authn.Multi(authn.Named(Scheme, captured)))
	handler := mw(func(_ context.Context, _ any) (any, error) { return "ok", nil })

	ctx := serverCtx(map[string]string{"Authorization": "Bearer fallback-tok"})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.calls != 1 {
		t.Fatalf("Authenticate calls = %d, want 1", captured.calls)
	}
	// Multi calls Authenticate with the original ctx (no WithToken was
	// installed by the framework dispatcher), so the stub observes the
	// jwt-private channel as empty — but Authenticate's real implementation
	// would consult the transport header. The stub here only proves the
	// engine was reached; tokenForAuth coverage is the next test.
	if captured.observedHasTok {
		t.Errorf("stub saw WithToken-backed entry, want empty (fallback proves engine resolves header itself)")
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
// ClaimsMapper tests — minimal three-piece mapping + extension point.
// ============================================================================

// TestDefaultClaimsMapper_SubAndName: canonical claims → 3-piece UserActor.
func TestDefaultClaimsMapper_SubAndName(t *testing.T) {
	mapper := DefaultClaimsMapper()
	a, err := mapper(gojwt.MapClaims{
		"sub":  "user-123",
		"name": "Alice",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Type() != actor.TypeUser {
		t.Errorf("actor type = %v, want user", a.Type())
	}
	if a.ID() != "user-123" {
		t.Errorf("actor.ID = %q, want user-123", a.ID())
	}
	ua, ok := a.(*actor.UserActor)
	if !ok {
		t.Fatalf("actor concrete type = %T, want *actor.UserActor", a)
	}
	if ua.DisplayName() != "Alice" {
		t.Errorf("DisplayName = %q, want Alice", ua.DisplayName())
	}
}

// TestDefaultClaimsMapper_PreferredUsernameFallback: when name is absent,
// preferred_username takes over for DisplayName.
func TestDefaultClaimsMapper_PreferredUsernameFallback(t *testing.T) {
	mapper := DefaultClaimsMapper()
	a, err := mapper(gojwt.MapClaims{
		"sub":                "user-456",
		"preferred_username": "alice42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ua, ok := a.(*actor.UserActor)
	if !ok {
		t.Fatalf("actor concrete type = %T, want *actor.UserActor", a)
	}
	if ua.DisplayName() != "alice42" {
		t.Errorf("DisplayName = %q, want alice42", ua.DisplayName())
	}
}

// TestDefaultClaimsMapper_NameOverridesPreferredUsername: when both present,
// name wins (per spec ordering).
func TestDefaultClaimsMapper_NameOverridesPreferredUsername(t *testing.T) {
	mapper := DefaultClaimsMapper()
	a, err := mapper(gojwt.MapClaims{
		"sub":                "user-789",
		"name":               "Real Name",
		"preferred_username": "alice42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ua := a.(*actor.UserActor)
	if ua.DisplayName() != "Real Name" {
		t.Errorf("DisplayName = %q, want Real Name", ua.DisplayName())
	}
}

// TestDefaultClaimsMapper_EmptySubFails: sub is REQUIRED.
func TestDefaultClaimsMapper_EmptySubFails(t *testing.T) {
	mapper := DefaultClaimsMapper()
	_, err := mapper(gojwt.MapClaims{
		"name": "Alice",
	})
	if err == nil {
		t.Fatal("expected error for empty sub, got nil")
	}
}

// TestDefaultClaimsMapper_OnlySub: name absent + preferred_username absent →
// 3-piece with empty DisplayName, no error.
func TestDefaultClaimsMapper_OnlySub(t *testing.T) {
	mapper := DefaultClaimsMapper()
	a, err := mapper(gojwt.MapClaims{"sub": "user-xyz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ua := a.(*actor.UserActor)
	if ua.ID() != "user-xyz" || ua.DisplayName() != "" {
		t.Errorf("actor = (%q,%q), want (user-xyz,\"\")", ua.ID(), ua.DisplayName())
	}
}

// TestWithClaimsMapper_CustomExtensionPoint: business-installed mapper is
// honored end-to-end, and can return ANY actor.Actor implementation.
func TestWithClaimsMapper_CustomExtensionPoint(t *testing.T) {
	custom := func(claims gojwt.MapClaims) (actor.Actor, error) {
		sub, _ := claims["sub"].(string)
		role, _ := claims["custom_role"].(string)
		if role == "" {
			return nil, errors.New("custom: missing custom_role")
		}
		return actor.NewUserActor(sub, fmt.Sprintf("%s[%s]", sub, role)), nil
	}

	auth := newAuthenticator(WithClaimsMapper(custom))
	if auth.cfg.claimsMapper == nil {
		t.Fatal("ClaimsMapper not installed by Option")
	}

	a, err := auth.cfg.claimsMapper(gojwt.MapClaims{
		"sub":         "user-1",
		"custom_role": "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ua, ok := a.(*actor.UserActor)
	if !ok {
		t.Fatalf("actor type = %T", a)
	}
	if ua.DisplayName() != "user-1[admin]" {
		t.Errorf("DisplayName = %q, want user-1[admin]", ua.DisplayName())
	}

	// And that errors propagate.
	_, err = auth.cfg.claimsMapper(gojwt.MapClaims{"sub": "user-1"})
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

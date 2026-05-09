package jwt

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
)

// ============================================================================
// Test fixtures: server / client transport fakes (jwt package-local).
//
// These fakes are intentionally duplicated from authn_test.go rather than
// cross-imported: keep test packages independent so a future move of
// jwt_test.go does not pull a hidden dep on authn package internals.
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
// headers AND a fresh audit detail holder (so audit.WithAuthnResult writes
// from the dispatcher land somewhere observable).
func serverCtx(headers map[string]string) context.Context {
	if headers == nil {
		headers = map[string]string{}
	}
	ctx := transport.NewServerContext(context.Background(), &fakeServerTransport{headers: headers})
	return audit.InstallHolder(ctx)
}

// clientCtx builds a client-side ctx with a fake outbound transport (used by
// jwt.Client tests).
func clientCtx() (context.Context, *fakeClientTransport) {
	tr := &fakeClientTransport{headers: map[string]string{}}
	return transport.NewClientContext(context.Background(), tr), tr
}

// ============================================================================
// Test fixtures: counting Authenticator (records call count for short-circuit
// assertions). Not exported.
// ============================================================================

type countingAuthenticator struct {
	calls       int
	returnActor actor.Actor
	returnErr   error
	// observedToken is set on each Authenticate call so tests can assert what
	// the engine actually saw via TokenFromContext.
	observedToken  string
	observedHasTok bool
}

func (c *countingAuthenticator) Authenticate(ctx context.Context) (actor.Actor, error) {
	c.calls++
	c.observedToken, c.observedHasTok = TokenFromContext(ctx)
	if c.returnErr != nil {
		return nil, c.returnErr
	}
	if c.returnActor == nil {
		return actor.NewAnonymousActor(), nil
	}
	return c.returnActor, nil
}

// serverWithStub builds Server() middleware backed by the supplied
// countingAuthenticator. Calls into the package-private serverWith(auth)
// composition so the test exercises the SAME wrapper shape that the public
// Server(opts...) constructor uses, without requiring a real Verifier dep.
func serverWithStub(stub *countingAuthenticator) middleware.Middleware {
	return serverWith(stub)
}

// ============================================================================
// New tests for the engine-agnostic refactor (Task 2).
// ============================================================================

// TestServer_ExtractsBearerAndDispatches asserts the wrapper:
//  1. reads Authorization: Bearer <tok> off the inbound transport,
//  2. stashes the raw token into the jwt-private ctx channel,
//  3. delegates to authn.Server which writes AuthnDetail.Method = "jwt".
func TestServer_ExtractsBearerAndDispatches(t *testing.T) {
	stub := &countingAuthenticator{
		returnActor: actor.NewUserActor(actor.UserActorParams{ID: "u1"}),
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

	// Handler ctx should also expose the same token via the jwt-private channel.
	tok, ok := TokenFromContext(capturedCtx)
	if !ok || tok != "raw-token-xyz" {
		t.Errorf("TokenFromContext(handler ctx) = (%q,%v), want (\"raw-token-xyz\",true)", tok, ok)
	}

	d, ok := audit.AuthnResultFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthnDetail in ctx")
	}
	if d.Method != "jwt" {
		t.Errorf("AuthnDetail.Method = %q, want jwt", d.Method)
	}
	if !d.Success {
		t.Errorf("AuthnDetail.Success = false, want true")
	}
}

// TestClient_PropagatesToken asserts the client middleware reads the token
// from the jwt-private ctx channel and sets the outbound Authorization header.
func TestClient_PropagatesToken(t *testing.T) {
	mw := Client()

	ctx, tr := clientCtx()
	ctx = WithToken(ctx, "abc123")

	handler := mw(func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})

	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := tr.headers["Authorization"]
	if got != "Bearer abc123" {
		t.Errorf("outbound Authorization = %q, want %q", got, "Bearer abc123")
	}
}

// TestServer_ChainShortCircuit_PassthroughOnExistingActor asserts the wrapper
// short-circuits when ctx already carries a non-anonymous actor: the
// Authenticator must NOT be invoked, the handler runs with the pre-set actor,
// and NO AuthnDetail is written (because the dispatcher was never reached).
func TestServer_ChainShortCircuit_PassthroughOnExistingActor(t *testing.T) {
	stub := &countingAuthenticator{
		returnActor: actor.NewUserActor(actor.UserActorParams{ID: "should-not-be-used"}),
	}
	mw := serverWithStub(stub)

	preSet := actor.NewUserActor(actor.UserActorParams{ID: "pre-set"})

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	// Ctx has a transport (so the wrapper could in theory extract a token) but
	// also has a non-anonymous actor pre-installed. Short-circuit must win.
	ctx := serverCtx(map[string]string{"Authorization": "Bearer would-be-token"})
	ctx = actor.NewContext(ctx, preSet)

	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stub.calls != 0 {
		t.Errorf("Authenticate calls = %d, want 0 (short-circuit)", stub.calls)
	}

	a, ok := actor.FromContext(capturedCtx)
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
// to the dispatcher: Authenticate runs, sees an empty token, and the
// dispatcher writes AuthnDetail{Method:"jwt", Success:true}.
func TestServer_NoBearerHeader_ReachesAuthenticator(t *testing.T) {
	stub := &countingAuthenticator{} // returnActor nil → anonymous
	mw := serverWithStub(stub)

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	})

	ctx := serverCtx(nil) // no Authorization header
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
	if d.Method != "jwt" {
		t.Errorf("AuthnDetail.Method = %q, want jwt", d.Method)
	}
	if !d.Success {
		t.Errorf("AuthnDetail.Success = false, want true (anonymous pass-through)")
	}
}

// ============================================================================
// Existing engine-level tests (Authenticate against the verifier) preserved
// from the pre-refactor file. These exercise NewAuthenticator + Verifier in
// isolation (no transport, no wrapper).
// ============================================================================

// TestAuthenticate_NoTokenInContext_ReturnsAnonymous: when no token is present
// in the jwt-private ctx channel, Authenticate returns an anonymous actor.
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

package apikey

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
)

// ============================================================================
// Test fixtures: server transport fake + stub Store. apikey-package-local.
// ============================================================================

type fakeServerTransport struct {
	headers map[string]string
}

func (f *fakeServerTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeServerTransport) Endpoint() string                { return "" }
func (f *fakeServerTransport) Operation() string               { return "" }
func (f *fakeServerTransport) RequestHeader() transport.Header { return &fakeHeader{f.headers} }
func (f *fakeServerTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

type fakeHeader struct {
	m map[string]string
}

func (h *fakeHeader) Get(key string) string    { return h.m[key] }
func (h *fakeHeader) Set(key, value string)    { h.m[key] = value }
func (h *fakeHeader) Add(_ string, _ string)   {}
func (h *fakeHeader) Keys() []string           { return nil }
func (h *fakeHeader) Values(_ string) []string { return nil }

// serverCtx builds a server-side ctx with a fake transport carrying the
// given headers.
func serverCtx(headers map[string]string) context.Context {
	if headers == nil {
		headers = map[string]string{}
	}
	return transport.NewServerContext(context.Background(), &fakeServerTransport{headers: headers})
}

// stubStore is a minimal in-memory [Store] used only by these tests; the
//灯塔 / e2e in-memory stub belongs to servora-example, not this package.
type stubStore struct {
	keys     map[string]actor.Actor
	forceErr error
	calls    int
	gotKey   string
}

func (s *stubStore) Lookup(_ context.Context, key string) (actor.Actor, error) {
	s.calls++
	s.gotKey = key
	if s.forceErr != nil {
		return nil, s.forceErr
	}
	a, ok := s.keys[key]
	if !ok {
		return nil, errors.New("apikey: unknown key")
	}
	return a, nil
}

// ============================================================================
// Construction tests.
// ============================================================================

// TestScheme_IsApikey: paranoid sanity check that the package-level
// constant exists and has the expected value (proto schemes lists rely on
// it being "apikey").
func TestScheme_IsApikey(t *testing.T) {
	if Scheme != "apikey" {
		t.Errorf("Scheme = %q, want apikey", Scheme)
	}
}

// TestNewAuthenticator_WithoutStore_Panics: fail-fast wiring guard. The
// engine cannot serve any request without a Store, so omitting WithStore
// MUST panic at construction time, NOT silently accept and 401 every
// request.
func TestNewAuthenticator_WithoutStore_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when WithStore is missing, got none")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value type = %T, want string", r)
		}
		if msg != "apikey: WithStore is required" {
			t.Errorf("panic message = %q, want %q", msg, "apikey: WithStore is required")
		}
	}()
	_ = NewAuthenticator()
}

// ============================================================================
// Authenticate tests.
// ============================================================================

// TestAuthenticate_MissingHeader_ReturnsError: ctx with transport but no
// X-API-Key header → engine returns (nil, error containing "missing
// X-API-Key") and Store.Lookup MUST NOT be called.
func TestAuthenticate_MissingHeader_ReturnsError(t *testing.T) {
	store := &stubStore{keys: map[string]actor.Actor{}}
	auth := NewAuthenticator(WithStore(store))

	ctx := serverCtx(nil)
	a, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error for missing X-API-Key, got nil")
	}
	if a != nil {
		t.Errorf("actor = %v, want nil on error", a)
	}
	if !strings.Contains(err.Error(), "missing X-API-Key") {
		t.Errorf("error = %q, want substring %q", err.Error(), "missing X-API-Key")
	}
	if store.calls != 0 {
		t.Errorf("Store.Lookup calls = %d, want 0 (header check must short-circuit)", store.calls)
	}
}

// TestAuthenticate_NoTransport_ReturnsError: bare context.Background() with
// no Kratos server transport attached → same missing-header error path
// (extractAPIKey returns "" → engine returns errMissingHeader).
func TestAuthenticate_NoTransport_ReturnsError(t *testing.T) {
	store := &stubStore{keys: map[string]actor.Actor{}}
	auth := NewAuthenticator(WithStore(store))

	a, err := auth.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error for missing transport, got nil")
	}
	if a != nil {
		t.Errorf("actor = %v, want nil on error", a)
	}
	if !strings.Contains(err.Error(), "missing X-API-Key") {
		t.Errorf("error = %q, want substring %q", err.Error(), "missing X-API-Key")
	}
	if store.calls != 0 {
		t.Errorf("Store.Lookup calls = %d, want 0", store.calls)
	}
}

// TestAuthenticate_HappyPath: valid X-API-Key resolves through Store to a
// concrete ServiceActor, returned verbatim.
func TestAuthenticate_HappyPath(t *testing.T) {
	wantActor := actor.NewServiceActor("svc-1", "Test Service")
	store := &stubStore{
		keys: map[string]actor.Actor{
			"valid": wantActor,
		},
	}
	auth := NewAuthenticator(WithStore(store))

	ctx := serverCtx(map[string]string{"X-API-Key": "valid"})
	a, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("actor = nil, want ServiceActor")
	}
	if a.ID() != "svc-1" {
		t.Errorf("actor.ID = %q, want svc-1", a.ID())
	}
	if a.Type() != actor.TypeService {
		t.Errorf("actor.Type = %v, want TypeService", a.Type())
	}
	if store.calls != 1 {
		t.Errorf("Store.Lookup calls = %d, want 1", store.calls)
	}
	if store.gotKey != "valid" {
		t.Errorf("Store.Lookup key = %q, want valid", store.gotKey)
	}
}

// TestAuthenticate_StoreError_Propagates: Store returns a specific error
// (e.g. unknown / revoked / disabled key) → engine propagates it verbatim
// (no wrapping), so middleware-level audit detail can surface the original
// reason string.
func TestAuthenticate_StoreError_Propagates(t *testing.T) {
	wantErr := errors.New("apikey: revoked key")
	store := &stubStore{forceErr: wantErr}
	auth := NewAuthenticator(WithStore(store))

	ctx := serverCtx(map[string]string{"X-API-Key": "anything"})
	a, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want errors.Is(%v) true", err, wantErr)
	}
	if a != nil {
		t.Errorf("actor = %v, want nil on error", a)
	}
}

// ============================================================================
// extractAPIKey tests (header carrier).
// ============================================================================

// TestExtractAPIKey_TransportPresent: header set on inbound transport →
// extractAPIKey returns the value.
func TestExtractAPIKey_TransportPresent(t *testing.T) {
	ctx := serverCtx(map[string]string{"X-API-Key": "abc"})
	got := extractAPIKey(ctx)
	if got != "abc" {
		t.Errorf("extractAPIKey = %q, want abc", got)
	}
}

// TestExtractAPIKey_NoTransport: bare ctx without server transport →
// returns "".
func TestExtractAPIKey_NoTransport(t *testing.T) {
	got := extractAPIKey(context.Background())
	if got != "" {
		t.Errorf("extractAPIKey = %q, want \"\"", got)
	}
}

// TestExtractAPIKey_HeaderAbsent: server transport present but no
// X-API-Key header set → returns "".
func TestExtractAPIKey_HeaderAbsent(t *testing.T) {
	ctx := serverCtx(map[string]string{"Authorization": "Bearer xyz"})
	got := extractAPIKey(ctx)
	if got != "" {
		t.Errorf("extractAPIKey = %q, want \"\" (Authorization is jwt's carrier, not apikey's)", got)
	}
}

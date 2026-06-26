package apikey

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/transport"

	"github.com/Servora-Kit/servora/security/authn"
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

// stubStore is a minimal in-memory [Store] used only by these tests.
type stubStore struct {
	keys     map[string]KeyMeta
	forceErr error
	calls    int
	gotKey   string
}

func (s *stubStore) Lookup(_ context.Context, key string) (KeyMeta, error) {
	s.calls++
	s.gotKey = key
	if s.forceErr != nil {
		return KeyMeta{}, s.forceErr
	}
	m, ok := s.keys[key]
	if !ok {
		return KeyMeta{}, errors.New("apikey: unknown key")
	}
	return m, nil
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
// X-API-Key header → engine returns (ctx, error containing "missing
// X-API-Key") and Store.Lookup MUST NOT be called.
func TestAuthenticate_MissingHeader_ReturnsError(t *testing.T) {
	store := &stubStore{keys: map[string]KeyMeta{}}
	auth := NewAuthenticator(WithStore(store))

	ctx := serverCtx(nil)
	resultCtx, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error for missing X-API-Key, got nil")
	}
	if resultCtx == nil {
		t.Error("ctx = nil, want non-nil (original ctx returned on error)")
	}
	if !strings.Contains(err.Error(), "missing X-API-Key") {
		t.Errorf("error = %q, want substring %q", err.Error(), "missing X-API-Key")
	}
	if !errors.Is(err, authn.ErrNoCredentials) {
		t.Errorf("error = %v, want ErrNoCredentials", err)
	}
	if store.calls != 0 {
		t.Errorf("Store.Lookup calls = %d, want 0 (header check must short-circuit)", store.calls)
	}
	// KeyMeta should NOT be present on error path.
	if _, ok := KeyMetaFrom(resultCtx); ok {
		t.Error("KeyMetaFrom should return false on error path")
	}
}

// TestAuthenticate_NoTransport_ReturnsError: bare context.Background() with
// no Kratos server transport attached → same missing-header error path
// (extractAPIKey returns "" → engine returns errMissingHeader).
func TestAuthenticate_NoTransport_ReturnsError(t *testing.T) {
	store := &stubStore{keys: map[string]KeyMeta{}}
	auth := NewAuthenticator(WithStore(store))

	resultCtx, err := auth.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error for missing transport, got nil")
	}
	if resultCtx == nil {
		t.Error("ctx = nil, want non-nil on error")
	}
	if !strings.Contains(err.Error(), "missing X-API-Key") {
		t.Errorf("error = %q, want substring %q", err.Error(), "missing X-API-Key")
	}
	if !errors.Is(err, authn.ErrNoCredentials) {
		t.Errorf("error = %v, want ErrNoCredentials", err)
	}
	if store.calls != 0 {
		t.Errorf("Store.Lookup calls = %d, want 0", store.calls)
	}
}

// TestAuthenticate_HappyPath: valid X-API-Key resolves through Store to a
// KeyMeta, which is attached to ctx along with auth type "api_key".
func TestAuthenticate_HappyPath(t *testing.T) {
	wantMeta := KeyMeta{KeyID: "key-001", OwnerID: "svc-1"}
	store := &stubStore{
		keys: map[string]KeyMeta{
			"valid": wantMeta,
		},
	}
	auth := NewAuthenticator(WithStore(store))

	ctx := serverCtx(map[string]string{"X-API-Key": "valid"})
	resultCtx, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resultCtx == nil {
		t.Fatal("ctx = nil, want enriched ctx")
	}

	// Verify KeyMeta is attached.
	meta, ok := KeyMetaFrom(resultCtx)
	if !ok {
		t.Fatal("KeyMetaFrom returned false, want true")
	}
	if meta.KeyID != "key-001" {
		t.Errorf("meta.KeyID = %q, want key-001", meta.KeyID)
	}
	if meta.OwnerID != "svc-1" {
		t.Errorf("meta.OwnerID = %q, want svc-1", meta.OwnerID)
	}

	// Verify auth type is set.
	authType, ok := authn.AuthTypeFrom(resultCtx)
	if !ok {
		t.Fatal("AuthTypeFrom returned false, want true")
	}
	if authType != "api_key" {
		t.Errorf("authType = %q, want api_key", authType)
	}

	// Verify SubjectFrom works.
	sub, ok := SubjectFrom(resultCtx)
	if !ok {
		t.Fatal("SubjectFrom returned false, want true")
	}
	if sub != "svc-1" {
		t.Errorf("SubjectFrom = %q, want svc-1", sub)
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
	resultCtx, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want errors.Is(%v) true", err, wantErr)
	}
	if errors.Is(err, authn.ErrNoCredentials) {
		t.Errorf("error = %v, must not match ErrNoCredentials", err)
	}
	if resultCtx == nil {
		t.Error("ctx = nil, want non-nil on error")
	}
	// KeyMeta should NOT be present on error path.
	if _, ok := KeyMetaFrom(resultCtx); ok {
		t.Error("KeyMetaFrom should return false on store error path")
	}
}

// ============================================================================
// SubjectFrom tests.
// ============================================================================

// TestSubjectFrom_Present: ctx with KeyMeta containing non-empty OwnerID.
func TestSubjectFrom_Present(t *testing.T) {
	ctx := WithKeyMeta(context.Background(), KeyMeta{KeyID: "k1", OwnerID: "user-42"})
	sub, ok := SubjectFrom(ctx)
	if !ok {
		t.Fatal("SubjectFrom returned false, want true")
	}
	if sub != "user-42" {
		t.Errorf("SubjectFrom = %q, want user-42", sub)
	}
}

// TestSubjectFrom_EmptyOwnerID: ctx with KeyMeta but OwnerID is empty →
// returns ("", false).
func TestSubjectFrom_EmptyOwnerID(t *testing.T) {
	ctx := WithKeyMeta(context.Background(), KeyMeta{KeyID: "k1", OwnerID: ""})
	sub, ok := SubjectFrom(ctx)
	if ok {
		t.Error("SubjectFrom returned true, want false for empty OwnerID")
	}
	if sub != "" {
		t.Errorf("SubjectFrom = %q, want \"\"", sub)
	}
}

// TestSubjectFrom_NoKeyMeta: bare ctx without KeyMeta → returns ("", false).
func TestSubjectFrom_NoKeyMeta(t *testing.T) {
	sub, ok := SubjectFrom(context.Background())
	if ok {
		t.Error("SubjectFrom returned true, want false for missing KeyMeta")
	}
	if sub != "" {
		t.Errorf("SubjectFrom = %q, want \"\"", sub)
	}
}

// ============================================================================
// KeyMetaFrom tests.
// ============================================================================

// TestKeyMetaFrom_Present: round-trip test for WithKeyMeta/KeyMetaFrom.
func TestKeyMetaFrom_Present(t *testing.T) {
	want := KeyMeta{KeyID: "k-abc", OwnerID: "owner-xyz"}
	ctx := WithKeyMeta(context.Background(), want)
	got, ok := KeyMetaFrom(ctx)
	if !ok {
		t.Fatal("KeyMetaFrom returned false, want true")
	}
	if got != want {
		t.Errorf("KeyMetaFrom = %+v, want %+v", got, want)
	}
}

// TestKeyMetaFrom_Absent: bare ctx → returns zero value and false.
func TestKeyMetaFrom_Absent(t *testing.T) {
	got, ok := KeyMetaFrom(context.Background())
	if ok {
		t.Error("KeyMetaFrom returned true, want false for bare ctx")
	}
	if got != (KeyMeta{}) {
		t.Errorf("KeyMetaFrom = %+v, want zero KeyMeta", got)
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

package authn

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// readFile is a thin os.ReadFile wrapper kept package-local so the structural
// guard tests do not pull in external file-reading helpers.
func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// fakeTransport implements transport.Transporter; only Operation matters for
// the dispatcher routing tests.
type fakeTransport struct {
	op string
}

func (f *fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string                { return "" }
func (f *fakeTransport) Operation() string               { return f.op }
func (f *fakeTransport) RequestHeader() transport.Header { return &fakeHeader{} }
func (f *fakeTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

type fakeHeader struct{}

func (h *fakeHeader) Get(key string) string      { return "" }
func (h *fakeHeader) Set(key, value string)      {}
func (h *fakeHeader) Add(key, value string)      {}
func (h *fakeHeader) Keys() []string             { return nil }
func (h *fakeHeader) Values(key string) []string { return nil }

// transportCtx builds a server-side ctx with a fake transport.
func transportCtx(op string) context.Context {
	return transport.NewServerContext(context.Background(), &fakeTransport{op: op})
}

// fakeAuthenticator records its invocation count and returns configured
// (ctx, err). Used everywhere the dispatcher (`Server`) is exercised
// without a Multi decorator.
type fakeAuthenticator struct {
	called    int
	returnCtx context.Context // if nil, returns the input ctx on success
	returnErr error

	// captureCtx, if non-nil, records the ctx received by Authenticate
	// so per-test assertions can inspect ctx channels installed by Server.
	captureCtx *context.Context
}

func (f *fakeAuthenticator) Authenticate(ctx context.Context) (context.Context, error) {
	f.called++
	if f.captureCtx != nil {
		*f.captureCtx = ctx
	}
	if f.returnErr != nil {
		return ctx, f.returnErr
	}
	if f.returnCtx != nil {
		return f.returnCtx, nil
	}
	return ctx, nil
}

// Compile-time guard: minimal Authenticator (single method only).
type minimalAuthenticator struct{}

func (minimalAuthenticator) Authenticate(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

var _ Authenticator = (*minimalAuthenticator)(nil)
var _ Authenticator = (*fakeAuthenticator)(nil)

// fakeAuditor captures CloudEvents emitted via WithAuditOnFailure.
type fakeAuditor struct {
	events []cloudevents.Event
}

func (a *fakeAuditor) Emit(_ context.Context, event cloudevents.Event) error {
	a.events = append(a.events, event)
	return nil
}

// ---------------------------------------------------------------------------
// Server: PublicMethods passthrough
// ---------------------------------------------------------------------------

func TestServer_PublicMethodsPassthrough(t *testing.T) {
	auth := &fakeAuthenticator{returnErr: errors.New("must not be called")}
	rules := func() Rules {
		return Rules{PublicMethods: []string{"/svc/Healthz"}}
	}

	mw := Server(auth, WithRulesFuncs(rules))
	handler := mw(func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})

	ctx := transportCtx("/svc/Healthz")
	resp, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
	if auth.called != 0 {
		t.Errorf("authenticator called %d times, want 0", auth.called)
	}
}

// ---------------------------------------------------------------------------
// Server: MethodSchemes path installs allowed set into ctx
// ---------------------------------------------------------------------------

func TestServer_MethodSchemes_InstallsAllowedSet(t *testing.T) {
	var capturedCtx context.Context
	auth := &fakeAuthenticator{
		captureCtx: &capturedCtx,
	}

	rules := func() Rules {
		return Rules{
			MethodSchemes: map[string][]string{
				"/svc/Op": {"jwt", "apikey"},
			},
		}
	}

	mw := Server(auth, WithRulesFuncs(rules))
	handler := mw(func(_ context.Context, _ any) (any, error) { return nil, nil })

	if _, err := handler(transportCtx("/svc/Op"), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.called != 1 {
		t.Fatalf("authenticator called %d times, want 1", auth.called)
	}

	allowed := allowedSchemesFrom(capturedCtx)
	if allowed == nil {
		t.Fatal("allowedSchemes ctx channel should be installed")
	}
	if _, ok := allowed["jwt"]; !ok {
		t.Error("allowed should contain jwt")
	}
	if _, ok := allowed["apikey"]; !ok {
		t.Error("allowed should contain apikey")
	}
	if _, ok := allowed["mtls"]; ok {
		t.Error("allowed should NOT contain mtls")
	}
}

// ---------------------------------------------------------------------------
// Server: unannotated path → allowed=nil (fail-open)
// ---------------------------------------------------------------------------

func TestServer_UnannotatedPath_AllowedNil(t *testing.T) {
	var capturedCtx context.Context
	auth := &fakeAuthenticator{
		captureCtx: &capturedCtx,
	}

	rules := func() Rules {
		// No PublicMethods or MethodSchemes entries for this op.
		return Rules{
			MethodSchemes: map[string][]string{"/svc/Other": {"jwt"}},
		}
	}

	mw := Server(auth, WithRulesFuncs(rules))
	handler := mw(func(_ context.Context, _ any) (any, error) { return nil, nil })

	if _, err := handler(transportCtx("/svc/Op"), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.called != 1 {
		t.Errorf("authenticator called %d times, want 1", auth.called)
	}
	if allowed := allowedSchemesFrom(capturedCtx); allowed != nil {
		t.Errorf("allowed = %v, want nil (fail-open)", allowed)
	}
}

// ---------------------------------------------------------------------------
// Server: success → enriched ctx passed to handler
// ---------------------------------------------------------------------------

func TestServer_Success_UsesEnrichedCtx(t *testing.T) {
	// The engine returns a ctx enriched with an auth type.
	enriched := WithAuthType(context.Background(), "jwt")
	inner := &fakeAuthenticator{returnCtx: enriched}
	auth := Multi(Named("jwt", inner))

	mw := Server(auth)

	var handlerCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		handlerCtx = ctx
		return nil, nil
	})

	if _, err := handler(transportCtx("/svc/Op"), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The handler should receive the enriched ctx from the engine.
	authType, ok := AuthTypeFrom(handlerCtx)
	if !ok {
		t.Fatal("expected AuthType in handler ctx (enriched from engine)")
	}
	if authType != "jwt" {
		t.Errorf("AuthType = %q, want jwt", authType)
	}
}

// ---------------------------------------------------------------------------
// Server: single-engine failure → default Unauthorized
// ---------------------------------------------------------------------------

func TestServer_SingleFailure_ReturnsUnauthorized(t *testing.T) {
	sentinel := errors.New("token expired")
	auth := &fakeAuthenticator{returnErr: sentinel}

	mw := Server(auth)
	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "AUTHN_FAILED") {
		t.Errorf("err = %q, expected to contain AUTHN_FAILED", err.Error())
	}
	if !strings.Contains(err.Error(), "token expired") {
		t.Errorf("err = %q, expected to contain underlying reason", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Server: Multi failure (SchemeAttemptsErr) → aggregated reason
// ---------------------------------------------------------------------------

func TestServer_MultiFailure_AggregatesReason(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnErr: ErrNoCredentials}
	apikeyAuth := &fakeAuthenticator{returnErr: ErrNoCredentials}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	rules := func() Rules {
		return Rules{
			MethodSchemes: map[string][]string{"/svc/Op": {"jwt", "apikey"}},
		}
	}

	mw := Server(auth,
		WithRulesFuncs(rules),
		WithErrorHandler(func(_ context.Context, err error) error {
			return err
		}),
	)

	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if err == nil {
		t.Fatal("expected error from Multi failure")
	}
	if _, ok := err.(SchemeAttemptsErr); !ok {
		t.Errorf("err type = %T, want SchemeAttemptsErr", err)
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "jwt: authn: no credentials") {
		t.Errorf("err = %q, missing jwt attempt", errStr)
	}
	if !strings.Contains(errStr, "apikey: authn: no credentials") {
		t.Errorf("err = %q, missing apikey attempt", errStr)
	}
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("err = %v, want ErrNoCredentials", err)
	}
}

// ---------------------------------------------------------------------------
// Server: WithErrorHandler overrides default response
// ---------------------------------------------------------------------------

func TestServer_WithErrorHandler_OverridesDefault(t *testing.T) {
	sentinel := errors.New("upstream failed")
	custom := errors.New("custom converted")

	auth := &fakeAuthenticator{returnErr: sentinel}
	mw := Server(auth, WithErrorHandler(func(_ context.Context, _ error) error {
		return custom
	}))

	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if !errors.Is(err, custom) {
		t.Errorf("err = %v, want %v", err, custom)
	}
}

// ---------------------------------------------------------------------------
// Server: default error response (no WithErrorHandler) is Unauthorized
// ---------------------------------------------------------------------------

func TestServer_DefaultErrorIsUnauthorized(t *testing.T) {
	sentinel := errors.New("boom")
	auth := &fakeAuthenticator{returnErr: sentinel}
	mw := Server(auth)

	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "AUTHN_FAILED") {
		t.Errorf("err = %q, expected to contain AUTHN_FAILED", err.Error())
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %q, expected to contain underlying reason 'boom'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Server: WithAuditOnFailure emits CloudEvent
// ---------------------------------------------------------------------------

func TestServer_WithAuditOnFailure_Emits(t *testing.T) {
	sentinel := errors.New("bad token")
	auth := &fakeAuthenticator{returnErr: sentinel}
	auditor := &fakeAuditor{}

	mw := Server(auth, WithAuditOnFailure(auditor))
	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, _ = handler(transportCtx("/svc/SecureOp"), nil)

	if len(auditor.events) != 1 {
		t.Fatalf("auditor.events count = %d, want 1", len(auditor.events))
	}
	evt := auditor.events[0]
	if evt.Type() != "servora.authn.v1.failure" {
		t.Errorf("event type = %q, want servora.authn.v1.failure", evt.Type())
	}
	if evt.Source() != "/svc/SecureOp" {
		t.Errorf("event source = %q, want /svc/SecureOp", evt.Source())
	}
	sev, _ := evt.Extensions()["severity"].(string)
	if sev != "WARN" {
		t.Errorf("event severity = %q, want WARN", sev)
	}
	// Data should contain the error reason.
	if data := string(evt.Data()); !strings.Contains(data, "bad token") {
		t.Errorf("event data = %q, want to contain 'bad token'", data)
	}
}

// ---------------------------------------------------------------------------
// Server: WithAuditOnFailure NOT configured → silent
// ---------------------------------------------------------------------------

func TestServer_WithoutAuditOnFailure_Silent(t *testing.T) {
	auth := &fakeAuthenticator{returnErr: errors.New("fail")}

	// No auditor configured — should not panic or emit.
	mw := Server(auth)
	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// No assertion on auditor — just ensure no panic.
}

// ---------------------------------------------------------------------------
// WithRulesFuncs merge behavior (variadic + nil + overwrite)
// ---------------------------------------------------------------------------

func TestWithRulesFuncs_MergeBehavior(t *testing.T) {
	fn1 := func() Rules {
		return Rules{
			PublicMethods: []string{"/a/Healthz"},
			MethodSchemes: map[string][]string{"/a/Op": {"jwt"}},
		}
	}
	fn2 := func() Rules {
		return Rules{
			PublicMethods: []string{"/b/Healthz"},
			MethodSchemes: map[string][]string{"/b/Op": {"apikey"}},
		}
	}

	cfg := &serverConfig{}
	WithRulesFuncs(fn1, nil, fn2)(cfg)

	if len(cfg.rules.PublicMethods) != 2 {
		t.Errorf("PublicMethods len = %d, want 2", len(cfg.rules.PublicMethods))
	}
	if cfg.rules.PublicMethods[0] != "/a/Healthz" || cfg.rules.PublicMethods[1] != "/b/Healthz" {
		t.Errorf("PublicMethods = %v, want [/a/Healthz /b/Healthz]", cfg.rules.PublicMethods)
	}
	if got := cfg.rules.MethodSchemes["/a/Op"]; len(got) != 1 || got[0] != "jwt" {
		t.Errorf("MethodSchemes[/a/Op] = %v, want [jwt]", got)
	}
	if got := cfg.rules.MethodSchemes["/b/Op"]; len(got) != 1 || got[0] != "apikey" {
		t.Errorf("MethodSchemes[/b/Op] = %v, want [apikey]", got)
	}
}

func TestWithRulesFuncs_LaterOverwritesEarlier(t *testing.T) {
	fn1 := func() Rules {
		return Rules{MethodSchemes: map[string][]string{"/svc/Op": {"jwt"}}}
	}
	fn2 := func() Rules {
		return Rules{MethodSchemes: map[string][]string{"/svc/Op": {"apikey"}}}
	}

	cfg := &serverConfig{}
	WithRulesFuncs(fn1, fn2)(cfg)

	got := cfg.rules.MethodSchemes["/svc/Op"]
	if len(got) != 1 || got[0] != "apikey" {
		t.Errorf("MethodSchemes[/svc/Op] = %v, want [apikey] (later wins)", got)
	}
}

// ---------------------------------------------------------------------------
// Multi: first-success-wins; subsequent engines NOT called
// ---------------------------------------------------------------------------

func TestMulti_FirstSuccessWins(t *testing.T) {
	enriched := WithAuthType(context.Background(), "jwt")
	first := &fakeAuthenticator{returnCtx: enriched}
	second := &fakeAuthenticator{returnErr: errors.New("must not be called")}

	auth := Multi(
		Named("jwt", first),
		Named("apikey", second),
	)

	ctx := withAllowedSchemes(context.Background(), nil)
	resultCtx, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the enriched ctx is returned.
	if at, ok := AuthTypeFrom(resultCtx); !ok || at != "jwt" {
		t.Errorf("AuthType from result = (%q, %v), want (jwt, true)", at, ok)
	}
	if first.called != 1 {
		t.Errorf("first.called = %d, want 1", first.called)
	}
	if second.called != 0 {
		t.Errorf("second.called = %d, want 0 (first-success short-circuit)", second.called)
	}
}

// ---------------------------------------------------------------------------
// Multi: allowed filter skips non-matching engines
// ---------------------------------------------------------------------------

func TestMulti_AllowedFilter_SkipsNonMatching(t *testing.T) {
	jwtAuth := &fakeAuthenticator{}
	apikeyAuth := &fakeAuthenticator{returnErr: errors.New("must not be called")}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	allowed := map[string]struct{}{"jwt": {}}
	ctx := withAllowedSchemes(context.Background(), allowed)

	if _, err := auth.Authenticate(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jwtAuth.called != 1 {
		t.Errorf("jwtAuth.called = %d, want 1", jwtAuth.called)
	}
	if apikeyAuth.called != 0 {
		t.Errorf("apikeyAuth.called = %d, want 0 (filtered out)", apikeyAuth.called)
	}
}

// ---------------------------------------------------------------------------
// Multi: empty intersection → errSchemesEmpty
// ---------------------------------------------------------------------------

func TestMulti_EmptyIntersection_ReturnsErrSchemesEmpty(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnErr: errors.New("must not be called")}

	auth := Multi(Named("jwt", jwtAuth))

	allowed := map[string]struct{}{"mtls": {}}
	ctx := withAllowedSchemes(context.Background(), allowed)

	_, err := auth.Authenticate(ctx)
	if !errors.Is(err, errSchemesEmpty) {
		t.Errorf("err = %v, want errSchemesEmpty", err)
	}
	if jwtAuth.called != 0 {
		t.Errorf("jwtAuth.called = %d, want 0 (filtered out)", jwtAuth.called)
	}
}

// ---------------------------------------------------------------------------
// Multi: all no credentials aggregate into SchemeAttemptsErr and ErrNoCredentials
// ---------------------------------------------------------------------------

func TestMulti_AllNoCredentials_AggregatesIntoSchemeAttemptsErr(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnErr: ErrNoCredentials}
	apikeyAuth := &fakeAuthenticator{returnErr: fmt.Errorf("apikey: %w", ErrNoCredentials)}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	ctx := withAllowedSchemes(context.Background(), nil)
	_, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}

	as, ok := err.(SchemeAttemptsErr)
	if !ok {
		t.Fatalf("err type = %T, want SchemeAttemptsErr", err)
	}
	attempts := as.SchemeAttempts()
	if len(attempts) != 2 {
		t.Fatalf("attempts len = %d, want 2", len(attempts))
	}
	if attempts[0].Scheme != "jwt" || attempts[0].Reason != "authn: no credentials" {
		t.Errorf("attempts[0] = %+v, want {jwt, authn: no credentials}", attempts[0])
	}
	if attempts[1].Scheme != "apikey" || attempts[1].Reason != "apikey: authn: no credentials" {
		t.Errorf("attempts[1] = %+v, want {apikey, apikey: authn: no credentials}", attempts[1])
	}
}

func TestMulti_InvalidCredentialFailsFast(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnErr: errors.New("jwt verify failed")}
	apikeyAuth := &fakeAuthenticator{}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	_, err := auth.Authenticate(withAllowedSchemes(context.Background(), nil))
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, must not match ErrNoCredentials", err)
	}
	if apikeyAuth.called != 0 {
		t.Fatalf("apikeyAuth.called = %d, want 0", apikeyAuth.called)
	}
}

// ---------------------------------------------------------------------------
// SchemeAttemptsErr interface assertion works on *schemeAttemptsErr
// ---------------------------------------------------------------------------

func TestSchemeAttemptsErr_InterfaceAssertion(t *testing.T) {
	pkgPrivate := &schemeAttemptsErr{
		attempts: []SchemeAttempt{
			{Scheme: "jwt", Reason: "boom"},
		},
	}

	var asInterface SchemeAttemptsErr = pkgPrivate
	if got := asInterface.SchemeAttempts(); len(got) != 1 || got[0].Scheme != "jwt" {
		t.Errorf("SchemeAttempts() = %v, want [{jwt boom}]", got)
	}
	// Also satisfies error interface.
	var asErr error = pkgPrivate
	if asErr.Error() == "" {
		t.Error("Error() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// Multi: iteration order follows injection order, NOT allowed order
// ---------------------------------------------------------------------------

func TestMulti_IterationFollowsInjectionOrderNotAllowedOrder(t *testing.T) {
	var callOrder []string
	jwtAuth := &fakeAuthenticator{returnErr: ErrNoCredentials}
	apikeyAuth := &fakeAuthenticator{returnErr: ErrNoCredentials}

	// Wrap each fake so we can observe call order.
	tracedJWT := authenticatorFunc(func(ctx context.Context) (context.Context, error) {
		callOrder = append(callOrder, "jwt")
		return jwtAuth.Authenticate(ctx)
	})
	tracedAPIKey := authenticatorFunc(func(ctx context.Context) (context.Context, error) {
		callOrder = append(callOrder, "apikey")
		return apikeyAuth.Authenticate(ctx)
	})

	// Injection order: jwt first.
	auth := Multi(
		Named("jwt", tracedJWT),
		Named("apikey", tracedAPIKey),
	)

	// Allowed map iteration is not ordered, but the injection order should
	// be the observable iteration order.
	allowed := map[string]struct{}{"apikey": {}, "jwt": {}}
	ctx := withAllowedSchemes(context.Background(), allowed)

	_, _ = auth.Authenticate(ctx)

	if len(callOrder) != 2 {
		t.Fatalf("callOrder len = %d, want 2", len(callOrder))
	}
	if callOrder[0] != "jwt" || callOrder[1] != "apikey" {
		t.Errorf("callOrder = %v, want [jwt apikey] (injection order)", callOrder)
	}
}

// authenticatorFunc adapts a func to the Authenticator interface for tests.
type authenticatorFunc func(ctx context.Context) (context.Context, error)

func (f authenticatorFunc) Authenticate(ctx context.Context) (context.Context, error) {
	return f(ctx)
}

// ---------------------------------------------------------------------------
// Compile-time guard: WithMethod has been removed
// ---------------------------------------------------------------------------

func TestWithMethod_Removed(t *testing.T) {
	body := mustReadFile(t, "authn.go")
	if strings.Contains(body, "func WithMethod(") {
		t.Error("authn.go MUST NOT define WithMethod after Multi/Named refactor")
	}
}

func TestNamed_ValidationPanics(t *testing.T) {
	assertPanic(t, func() { Named("", &fakeAuthenticator{}) })
	assertPanic(t, func() { Named("jwt", nil) })
}

func TestMulti_ValidationPanics(t *testing.T) {
	assertPanic(t, func() { Multi() })
	assertPanic(t, func() {
		Multi(
			Named("jwt", &fakeAuthenticator{}),
			Named("jwt", &fakeAuthenticator{}),
		)
	})
	assertPanic(t, func() {
		Multi(NamedAuthenticator{scheme: "jwt"})
	})
	assertPanic(t, func() {
		Multi(NamedAuthenticator{inner: &fakeAuthenticator{}})
	})
}

func TestSuccessfulSchemeHolder_Removed(t *testing.T) {
	body := mustReadFile(t, "context.go")
	for _, needle := range []string{"successfulSchemeKey", "schemeHolder", "installSchemeHolder"} {
		if strings.Contains(body, needle) {
			t.Errorf("context.go MUST NOT contain %s", needle)
		}
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

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := readFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Ensure unused imports are not present (satisfy the Go compiler).
var _ middleware.Middleware

package authn

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
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

// transportCtx builds a server-side ctx with a fake transport plus a fresh
// audit detail holder (mirrors what audit.Collector installs in production).
func transportCtx(op string) context.Context {
	ctx := transport.NewServerContext(context.Background(), &fakeTransport{op: op})
	return audit.InstallHolder(ctx)
}

// fakeAuthenticator records its invocation count and returns configured
// (actor, err). Used everywhere the dispatcher (`Server`) is exercised
// without a Multi decorator.
type fakeAuthenticator struct {
	called      int
	returnActor actor.Actor
	returnErr   error

	// captureCtx, if non-nil, records the ctx received by Authenticate
	// so per-test assertions can inspect ctx channels installed by Server.
	captureCtx *context.Context
}

func (f *fakeAuthenticator) Authenticate(ctx context.Context) (actor.Actor, error) {
	f.called++
	if f.captureCtx != nil {
		*f.captureCtx = ctx
	}
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	if f.returnActor == nil {
		return actor.NewAnonymousActor(), nil
	}
	return f.returnActor, nil
}

// Compile-time guard: minimal Authenticator (single method only).
type minimalAuthenticator struct{}

func (minimalAuthenticator) Authenticate(_ context.Context) (actor.Actor, error) {
	return actor.NewAnonymousActor(), nil
}

var _ Authenticator = (*minimalAuthenticator)(nil)
var _ Authenticator = (*fakeAuthenticator)(nil)

// captureEmitter is a minimal audit.Emitter used in the end-to-end Collector
// assembly test. Mirrored locally to keep test packages independent.
type captureEmitter struct {
	events []*auditpb.AuditEvent
}

func (e *captureEmitter) Emit(_ context.Context, event *auditpb.AuditEvent) error {
	e.events = append(e.events, event)
	return nil
}

func (e *captureEmitter) Close() error { return nil }

// ---------------------------------------------------------------------------
// Server: chain short-circuit on existing non-anonymous actor
// ---------------------------------------------------------------------------

func TestServer_ChainShortCircuit_PreExistingActor(t *testing.T) {
	auth := &fakeAuthenticator{returnErr: errors.New("must not be called")}
	mw := Server(auth)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.From(ctx)
		if !ok {
			t.Fatal("expected pre-existing actor in ctx")
		}
		if a.ID() != "u-existing" {
			t.Errorf("actor.ID = %q, want u-existing", a.ID())
		}
		return "ok", nil
	})

	ctx := actor.NewContext(transportCtx("/svc/Op"), actor.NewUserActor("u-existing", "Existing"))
	resp, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
	if auth.called != 0 {
		t.Errorf("authenticator called %d times, want 0 (chain short-circuit)", auth.called)
	}
	if _, ok := audit.AuthnResultFrom(ctx); ok {
		t.Error("AuthnDetail should NOT be written on chain short-circuit")
	}
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
	handler := mw(func(ctx context.Context, req any) (any, error) {
		if _, ok := actor.From(ctx); ok {
			t.Error("PublicMethods passthrough must NOT inject an actor")
		}
		return "ok", nil
	})

	ctx := transportCtx("/svc/Healthz")
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.called != 0 {
		t.Errorf("authenticator called %d times, want 0", auth.called)
	}
	if _, ok := audit.AuthnResultFrom(ctx); ok {
		t.Error("AuthnDetail should NOT be written on PublicMethods passthrough")
	}
}

// ---------------------------------------------------------------------------
// Server: MethodSchemes path installs allowed set into ctx
// ---------------------------------------------------------------------------

func TestServer_MethodSchemes_InstallsAllowedSet(t *testing.T) {
	var capturedCtx context.Context
	auth := &fakeAuthenticator{
		returnActor: actor.NewUserActor("u1", "Test"),
		captureCtx:  &capturedCtx,
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
		returnActor: actor.NewUserActor("u1", "Test"),
		captureCtx:  &capturedCtx,
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
	if h := schemeHolderFrom(capturedCtx); h == nil {
		t.Error("schemeHolder should be installed even on fail-open path")
	}
}

// ---------------------------------------------------------------------------
// Server: success → AuthnDetail written + actor injected
// ---------------------------------------------------------------------------

func TestServer_Success_WritesDetailAndInjectsActor(t *testing.T) {
	user := actor.NewUserActor("u1", "Test")

	// Wrap engine in Multi(Named) so a scheme name flows into the holder
	// (which Server then writes into AuthnDetail.Method).
	inner := &fakeAuthenticator{returnActor: user}
	auth := Multi(Named("jwt", inner))

	mw := Server(auth)

	var capturedCtx context.Context
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		capturedCtx = ctx
		a, ok := actor.From(ctx)
		if !ok {
			t.Fatal("expected actor injected into ctx")
		}
		if a.ID() != "u1" {
			t.Errorf("actor.ID = %q, want u1", a.ID())
		}
		return nil, nil
	})

	if _, err := handler(transportCtx("/svc/Op"), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d, ok := audit.AuthnResultFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthnDetail written to ctx on success")
	}
	if d.Method != "jwt" {
		t.Errorf("Method = %q, want jwt (from holder)", d.Method)
	}
	if !d.Success {
		t.Error("Success = false, want true")
	}
	if d.FailureReason != "" {
		t.Errorf("FailureReason = %q, want empty", d.FailureReason)
	}
}

// ---------------------------------------------------------------------------
// Server: single-engine failure → AuthnDetail with err.Error()
// ---------------------------------------------------------------------------

func TestServer_SingleFailure_WritesDetail(t *testing.T) {
	sentinel := errors.New("token expired")
	auth := &fakeAuthenticator{returnErr: sentinel}

	var capturedCtx context.Context
	mw := Server(auth, WithErrorHandler(func(ctx context.Context, err error) error {
		capturedCtx = ctx
		return err
	}))

	handler := mw(func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler must not run on auth failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}

	d, ok := audit.AuthnResultFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthnDetail written before error return")
	}
	// No Multi → holder empty → Method empty (single-engine direct mount).
	if d.Method != "" {
		t.Errorf("Method = %q, want empty (no Multi → holder unwritten)", d.Method)
	}
	if d.Success {
		t.Error("Success = true, want false")
	}
	if d.FailureReason != sentinel.Error() {
		t.Errorf("FailureReason = %q, want %q", d.FailureReason, sentinel.Error())
	}
}

// ---------------------------------------------------------------------------
// Server: Multi failure (SchemeAttemptsErr) → Method=multi, aggregated reason
// ---------------------------------------------------------------------------

func TestServer_MultiFailure_AggregatesReason(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnErr: errors.New("jwt verify failed")}
	apikeyAuth := &fakeAuthenticator{returnErr: errors.New("missing X-API-Key")}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	rules := func() Rules {
		return Rules{
			MethodSchemes: map[string][]string{"/svc/Op": {"jwt", "apikey"}},
		}
	}

	var capturedCtx context.Context
	mw := Server(auth,
		WithRulesFuncs(rules),
		WithErrorHandler(func(ctx context.Context, err error) error {
			capturedCtx = ctx
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

	d, ok := audit.AuthnResultFrom(capturedCtx)
	if !ok {
		t.Fatal("expected AuthnDetail written before error return")
	}
	if d.Method != "multi" {
		t.Errorf("Method = %q, want multi", d.Method)
	}
	if d.Success {
		t.Error("Success = true, want false")
	}
	if !strings.Contains(d.FailureReason, "jwt: jwt verify failed") {
		t.Errorf("FailureReason = %q, missing jwt attempt", d.FailureReason)
	}
	if !strings.Contains(d.FailureReason, "apikey: missing X-API-Key") {
		t.Errorf("FailureReason = %q, missing apikey attempt", d.FailureReason)
	}
	if !strings.Contains(d.FailureReason, ";") {
		t.Errorf("FailureReason = %q, expected attempts joined by '; '", d.FailureReason)
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
	// Kratos errors.Unauthorized produces an *errors.Error with reason
	// "AUTHN_FAILED" and message containing the underlying err string.
	if !strings.Contains(err.Error(), "AUTHN_FAILED") {
		t.Errorf("err = %q, expected to contain AUTHN_FAILED", err.Error())
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %q, expected to contain underlying reason 'boom'", err.Error())
	}
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
	first := &fakeAuthenticator{returnActor: actor.NewUserActor("u1", "First")}
	second := &fakeAuthenticator{returnErr: errors.New("must not be called")}

	auth := Multi(
		Named("jwt", first),
		Named("apikey", second),
	)

	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), nil))
	a, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil || a.ID() != "u1" {
		t.Errorf("actor = %v, want id=u1", a)
	}
	if first.called != 1 {
		t.Errorf("first.called = %d, want 1", first.called)
	}
	if second.called != 0 {
		t.Errorf("second.called = %d, want 0 (first-success short-circuit)", second.called)
	}
}

// ---------------------------------------------------------------------------
// Multi: anonymous-fallthrough sub-success is treated as soft failure
// ---------------------------------------------------------------------------

// TestMulti_AnonymousSuccess_TreatedAsSoftFailure verifies that an engine
// returning (AnonymousActor, nil) — the "no credential, passthrough" pattern
// from single-engine direct mounts — does NOT short-circuit Multi. The next
// allowed engine is given a chance to produce a concrete identity.
//
// Without this guard, jwt's "no Authorization header → anonymous + nil err"
// would silently win first-success-wins and starve subsequent engines (e.g.
// apikey), defeating the multi-scheme contract enforced by MODE_REQUIRED
// schemes=[jwt,apikey].
func TestMulti_AnonymousSuccess_TreatedAsSoftFailure(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnActor: actor.NewAnonymousActor()}
	apikeyAuth := &fakeAuthenticator{returnActor: actor.NewServiceActor("svc-1", "Service One")}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), nil))
	a, err := auth.Authenticate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil || a.ID() != "svc-1" {
		t.Errorf("actor = %v, want id=svc-1 (apikey wins after jwt anonymous fallthrough)", a)
	}
	if jwtAuth.called != 1 {
		t.Errorf("jwtAuth.called = %d, want 1", jwtAuth.called)
	}
	if apikeyAuth.called != 1 {
		t.Errorf("apikeyAuth.called = %d, want 1 (anonymous fallthrough must not short-circuit)", apikeyAuth.called)
	}
	if h := schemeHolderFrom(ctx); h == nil || h.scheme != "apikey" {
		t.Errorf("holder scheme = %q, want apikey", h.scheme)
	}
}

// TestMulti_AnonymousOnlyEngines_AllFail verifies that when EVERY engine
// returns anonymous, Multi reports schemeAttemptsErr (not silent success).
// The aggregate FailureReason carries one "no credential ..." entry per
// engine, so audit logs can distinguish "no token sent" from "bad token".
func TestMulti_AnonymousOnlyEngines_AllFail(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnActor: actor.NewAnonymousActor()}
	apikeyAuth := &fakeAuthenticator{returnActor: actor.NewAnonymousActor()}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), nil))
	a, err := auth.Authenticate(ctx)
	if a != nil {
		t.Errorf("actor = %v, want nil", a)
	}
	as, ok := err.(SchemeAttemptsErr)
	if !ok {
		t.Fatalf("err = %v, want SchemeAttemptsErr", err)
	}
	attempts := as.SchemeAttempts()
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(attempts))
	}
	for _, at := range attempts {
		if !strings.Contains(at.Reason, "no credential") {
			t.Errorf("attempt %q reason = %q, want contains 'no credential'", at.Scheme, at.Reason)
		}
	}
}

// ---------------------------------------------------------------------------
// Multi: allowed filter skips non-matching engines
// ---------------------------------------------------------------------------

func TestMulti_AllowedFilter_SkipsNonMatching(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnActor: actor.NewUserActor("u1", "JWT")}
	apikeyAuth := &fakeAuthenticator{returnErr: errors.New("must not be called")}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	allowed := map[string]struct{}{"jwt": {}}
	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), allowed))

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
	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), allowed))

	_, err := auth.Authenticate(ctx)
	if !errors.Is(err, errSchemesEmpty) {
		t.Errorf("err = %v, want errSchemesEmpty", err)
	}
	if jwtAuth.called != 0 {
		t.Errorf("jwtAuth.called = %d, want 0 (filtered out)", jwtAuth.called)
	}
}

// ---------------------------------------------------------------------------
// Multi: writes scheme to holder on success
// ---------------------------------------------------------------------------

func TestMulti_WritesSchemeToHolderOnSuccess(t *testing.T) {
	first := &fakeAuthenticator{returnErr: errors.New("first failed")}
	second := &fakeAuthenticator{returnActor: actor.NewUserActor("u1", "Second")}

	auth := Multi(
		Named("jwt", first),
		Named("apikey", second),
	)

	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), nil))
	if _, err := auth.Authenticate(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	holder := schemeHolderFrom(ctx)
	if holder == nil {
		t.Fatal("holder must be present")
	}
	if got := holder.get(); got != "apikey" {
		t.Errorf("holder.get() = %q, want apikey", got)
	}
}

// ---------------------------------------------------------------------------
// Multi: aggregates failures into SchemeAttemptsErr
// ---------------------------------------------------------------------------

func TestMulti_AllFailed_AggregatesIntoSchemeAttemptsErr(t *testing.T) {
	jwtAuth := &fakeAuthenticator{returnErr: errors.New("jwt verify failed")}
	apikeyAuth := &fakeAuthenticator{returnErr: errors.New("missing X-API-Key")}

	auth := Multi(
		Named("jwt", jwtAuth),
		Named("apikey", apikeyAuth),
	)

	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), nil))
	_, err := auth.Authenticate(ctx)
	if err == nil {
		t.Fatal("expected aggregated error")
	}

	as, ok := err.(SchemeAttemptsErr)
	if !ok {
		t.Fatalf("err type = %T, want SchemeAttemptsErr", err)
	}
	attempts := as.SchemeAttempts()
	if len(attempts) != 2 {
		t.Fatalf("attempts len = %d, want 2", len(attempts))
	}
	if attempts[0].Scheme != "jwt" || attempts[0].Reason != "jwt verify failed" {
		t.Errorf("attempts[0] = %+v, want {jwt, jwt verify failed}", attempts[0])
	}
	if attempts[1].Scheme != "apikey" || attempts[1].Reason != "missing X-API-Key" {
		t.Errorf("attempts[1] = %+v, want {apikey, missing X-API-Key}", attempts[1])
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
	jwtAuth := &fakeAuthenticator{returnErr: errors.New("jwt failed")}
	apikeyAuth := &fakeAuthenticator{returnErr: errors.New("apikey failed")}

	// Wrap each fake so we can observe call order.
	tracedJWT := authenticatorFunc(func(ctx context.Context) (actor.Actor, error) {
		callOrder = append(callOrder, "jwt")
		return jwtAuth.Authenticate(ctx)
	})
	tracedAPIKey := authenticatorFunc(func(ctx context.Context) (actor.Actor, error) {
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
	ctx := installSchemeHolder(withAllowedSchemes(context.Background(), allowed))

	_, _ = auth.Authenticate(ctx)

	if len(callOrder) != 2 {
		t.Fatalf("callOrder len = %d, want 2", len(callOrder))
	}
	if callOrder[0] != "jwt" || callOrder[1] != "apikey" {
		t.Errorf("callOrder = %v, want [jwt apikey] (injection order)", callOrder)
	}
}

// authenticatorFunc adapts a func to the Authenticator interface for tests.
type authenticatorFunc func(ctx context.Context) (actor.Actor, error)

func (f authenticatorFunc) Authenticate(ctx context.Context) (actor.Actor, error) {
	return f(ctx)
}

// ---------------------------------------------------------------------------
// End-to-end: outer Collector still emits AUTHN_RESULT on authn failure
// ---------------------------------------------------------------------------

func TestServer_FailurePath_EmitsViaOuterCollector(t *testing.T) {
	emitter := &captureEmitter{}
	rec := audit.NewRecorder(emitter, "test-svc")

	sentinel := errors.New("auth failed")
	failAuth := &fakeAuthenticator{returnErr: sentinel}

	chain := middleware.Chain(audit.Collector(rec), Server(failAuth))
	handler := chain(func(_ context.Context, _ any) (any, error) {
		t.Fatal("inner handler must not run on authn failure")
		return nil, nil
	})

	_, err := handler(transportCtx("/svc/Op"), nil)
	if err == nil {
		t.Fatal("expected error from authn failure")
	}

	if len(emitter.events) != 1 {
		t.Fatalf("emit count = %d, want 1", len(emitter.events))
	}
	evt := emitter.events[0]
	if evt.GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT {
		t.Errorf("EventType = %v, want AUTHN_RESULT", evt.GetEventType())
	}
	d := evt.GetAuthnDetail()
	if d == nil {
		t.Fatal("AuthnDetail missing in emitted event")
	}
	if d.GetSuccess() {
		t.Error("AuthnDetail.Success = true on authn-failure event")
	}
	if d.GetFailureReason() != sentinel.Error() {
		t.Errorf("AuthnDetail.FailureReason = %q, want %q", d.GetFailureReason(), sentinel.Error())
	}
	if evt.GetResult().GetSuccess() {
		t.Error("Result.Success = true on authn-failure event")
	}
}

// ---------------------------------------------------------------------------
// Compile-time guard: WithMethod has been removed
// ---------------------------------------------------------------------------

// If a future commit re-adds WithMethod, this file remains compilable but the
// behavior of the rest of the package would diverge from spec. The grep-based
// guard below catches that explicitly.

func TestWithMethod_Removed(t *testing.T) {
	// Reflectively assert that no exported symbol named WithMethod exists by
	// reading the source file. A direct compile-time reference would tie the
	// test to the very symbol we want to ensure is gone.
	body := mustReadFile(t, "authn.go")
	if strings.Contains(body, "func WithMethod(") {
		t.Error("authn.go MUST NOT define WithMethod after Multi/Named refactor")
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := readFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

package authn

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// fakeTransport implements transport.Transporter for test purposes.
type fakeTransport struct {
	headers map[string]string
}

func (f *fakeTransport) Kind() transport.Kind             { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string                 { return "" }
func (f *fakeTransport) Operation() string                { return "" }
func (f *fakeTransport) RequestHeader() transport.Header  { return &fakeHeader{f.headers} }
func (f *fakeTransport) ReplyHeader() transport.Header    { return &fakeHeader{} }

type fakeHeader struct {
	m map[string]string
}

func (h *fakeHeader) Get(key string) string      { return h.m[key] }
func (h *fakeHeader) Set(key, value string)      { h.m[key] = value }
func (h *fakeHeader) Add(key, value string)      {}
func (h *fakeHeader) Keys() []string             { return nil }
func (h *fakeHeader) Values(key string) []string { return nil }

func transportCtx(headers map[string]string) context.Context {
	return transport.NewServerContext(context.Background(), &fakeTransport{headers: headers})
}

// fakeAuthenticator is a minimal Authenticator for unit tests.
// `method` allows tests to verify the middleware writes engine.Method() into
// ctx instead of hard-coding a scheme. Empty `method` defaults to "jwt" for
// backwards-compatibility with the legacy non-Method-aware tests.
type fakeAuthenticator struct {
	method      string
	returnActor actor.Actor
	returnErr   error
}

func (f *fakeAuthenticator) Method() string {
	if f.method == "" {
		return "jwt"
	}
	return f.method
}

func (f *fakeAuthenticator) Authenticate(_ context.Context) (actor.Actor, error) {
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	if f.returnActor == nil {
		return actor.NewAnonymousActor(), nil
	}
	return f.returnActor, nil
}

// TestServer_NoTransport_AnonymousActor checks that without a transport context an
// anonymous actor is injected and the authenticator is not called.
func TestServer_NoTransport_AnonymousActor(t *testing.T) {
	auth := &fakeAuthenticator{}
	mw := Server(auth)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if a.Type() != actor.TypeAnonymous {
			t.Errorf("expected TypeAnonymous, got %v", a.Type())
		}
		return nil, nil
	})

	_, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestServer_NoToken_AuthenticatorCalled checks that with no token, the authenticator
// is still called and its result is used.
func TestServer_NoToken_AuthenticatorCalled(t *testing.T) {
	userActor := actor.NewUserActor(actor.UserActorParams{ID: "u1", DisplayName: "Test"})
	auth := &fakeAuthenticator{returnActor: userActor}
	mw := Server(auth)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if a.ID() != "u1" {
			t.Errorf("actor id = %q, want u1", a.ID())
		}
		return "ok", nil
	})

	ctx := transportCtx(map[string]string{})
	resp, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
}

// TestServer_WithToken_TokenStoredInContext checks that the raw Bearer token is stored
// in context for downstream consumers.
func TestServer_WithToken_TokenStoredInContext(t *testing.T) {
	auth := &fakeAuthenticator{}
	mw := Server(auth)

	const rawToken = "myrawtoken"
	handler := mw(func(ctx context.Context, req any) (any, error) {
		tok, hasTok := svrmw.TokenFromContext(ctx)
		if !hasTok {
			t.Fatal("expected token in context")
		}
		if tok != rawToken {
			t.Errorf("token = %q, want %q", tok, rawToken)
		}
		return nil, nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer " + rawToken})
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestServer_AuthenticatorError_Propagated checks that errors from the authenticator propagate.
func TestServer_AuthenticatorError_Propagated(t *testing.T) {
	sentinel := errors.New("auth failed")
	auth := &fakeAuthenticator{returnErr: sentinel}
	mw := Server(auth)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on auth error")
		return nil, nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
	_, err := handler(ctx, nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel", err)
	}
}

// TestServer_CustomErrorHandler_InvokedOnError checks that WithErrorHandler is used.
func TestServer_CustomErrorHandler_InvokedOnError(t *testing.T) {
	sentinel := errors.New("auth failed")
	customErr := errors.New("custom error")
	auth := &fakeAuthenticator{returnErr: sentinel}
	mw := Server(auth, WithErrorHandler(func(_ context.Context, _ error) error { return customErr }))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on auth error")
		return nil, nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
	_, err := handler(ctx, nil)
	if !errors.Is(err, customErr) {
		t.Errorf("err = %v, want customErr", err)
	}
}

// TestServer_WritesAuthnResultToContext exercises every middleware exit path
// and asserts the AuthnDetail written to ctx matches expectations.
//
// Pipeline contract: middleware writes *auditpb.AuthnDetail to ctx via
// audit.WithAuthnResult; the transport-tail audit.Collector emits it.
// Failure path writes ctx BEFORE returning the error so collector can still
// observe it (only WithErrorHandler captures ctx in tests since handler
// never runs on failure).
func TestServer_WritesAuthnResultToContext(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userActor := actor.NewUserActor(actor.UserActorParams{ID: "u1", DisplayName: "Test"})
		auth := &fakeAuthenticator{returnActor: userActor}
		mw := Server(auth)

		var capturedCtx context.Context
		handler := mw(func(ctx context.Context, req any) (any, error) {
			capturedCtx = ctx
			return nil, nil
		})

		ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
		if _, err := handler(ctx, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		d, ok := audit.AuthnResultFrom(capturedCtx)
		if !ok {
			t.Fatal("expected AuthnDetail in ctx")
		}
		if d.Method != "jwt" {
			t.Errorf("Method = %q, want jwt", d.Method)
		}
		if !d.Success {
			t.Errorf("Success = false, want true")
		}
		if d.FailureReason != "" {
			t.Errorf("FailureReason = %q, want empty", d.FailureReason)
		}
	})

	t.Run("failure", func(t *testing.T) {
		sentinel := errors.New("auth failed")
		auth := &fakeAuthenticator{returnErr: sentinel}

		// Use WithErrorHandler purely to capture the ctx that the middleware
		// wrote to before returning the error. The handler itself never runs
		// on failure, so we cannot capture via the inner closure.
		var capturedCtx context.Context
		mw := Server(auth, WithErrorHandler(func(ctx context.Context, err error) error {
			capturedCtx = ctx
			return err
		}))

		handler := mw(func(ctx context.Context, req any) (any, error) {
			t.Fatal("handler should not run on auth failure")
			return nil, nil
		})

		ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
		if _, err := handler(ctx, nil); !errors.Is(err, sentinel) {
			t.Fatalf("err = %v, want sentinel", err)
		}

		d, ok := audit.AuthnResultFrom(capturedCtx)
		if !ok {
			t.Fatal("expected AuthnDetail in ctx (written before error return)")
		}
		if d.Method != "jwt" {
			t.Errorf("Method = %q, want jwt", d.Method)
		}
		if d.Success {
			t.Errorf("Success = true, want false")
		}
		if d.FailureReason != sentinel.Error() {
			t.Errorf("FailureReason = %q, want %q", d.FailureReason, sentinel.Error())
		}
	})

	t.Run("anonymous", func(t *testing.T) {
		auth := &fakeAuthenticator{returnActor: actor.NewAnonymousActor()}
		mw := Server(auth)

		var capturedCtx context.Context
		handler := mw(func(ctx context.Context, req any) (any, error) {
			capturedCtx = ctx
			return nil, nil
		})

		ctx := transportCtx(map[string]string{}) // no Authorization
		if _, err := handler(ctx, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		d, ok := audit.AuthnResultFrom(capturedCtx)
		if !ok {
			t.Fatal("expected AuthnDetail in ctx")
		}
		if d.Method != "jwt" {
			t.Errorf("Method = %q, want jwt", d.Method)
		}
		if !d.Success {
			t.Errorf("Success = false, want true")
		}
		if d.FailureReason != "" {
			t.Errorf("FailureReason = %q, want empty", d.FailureReason)
		}
	})

	t.Run("no-transport", func(t *testing.T) {
		auth := &fakeAuthenticator{}
		mw := Server(auth)

		var capturedCtx context.Context
		handler := mw(func(ctx context.Context, req any) (any, error) {
			capturedCtx = ctx
			return nil, nil
		})

		// No transport in ctx — early-return path. Treated as "anonymous
		// success" for symmetry with the in-engine missing-header path.
		if _, err := handler(context.Background(), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		d, ok := audit.AuthnResultFrom(capturedCtx)
		if !ok {
			t.Fatal("expected AuthnDetail in ctx for no-transport path")
		}
		if d.Method != "jwt" {
			t.Errorf("Method = %q, want jwt", d.Method)
		}
		if !d.Success {
			t.Errorf("Success = false, want true")
		}
		if d.FailureReason != "" {
			t.Errorf("FailureReason = %q, want empty", d.FailureReason)
		}
	})
}

// TestServer_MethodFromEngine is a regression guard: the Method string in the
// ctx-written AuthnDetail MUST come from authenticator.Method() — never
// hard-coded inside the middleware. If anyone reverts to a literal "jwt",
// the mtls/noop sub-cases will fail.
func TestServer_MethodFromEngine(t *testing.T) {
	cases := []struct {
		method string
	}{
		{"jwt"},
		{"mtls"},
		{"noop"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			auth := &fakeAuthenticator{
				method:      tc.method,
				returnActor: actor.NewAnonymousActor(),
			}
			mw := Server(auth)

			var capturedCtx context.Context
			handler := mw(func(ctx context.Context, req any) (any, error) {
				capturedCtx = ctx
				return nil, nil
			})

			ctx := transportCtx(map[string]string{})
			if _, err := handler(ctx, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			d, ok := audit.AuthnResultFrom(capturedCtx)
			if !ok {
				t.Fatal("expected AuthnDetail in ctx")
			}
			if d.Method != tc.method {
				t.Errorf("Method = %q, want %q (engine is source of truth)", d.Method, tc.method)
			}
		})
	}
}

// captureEmitter is a minimal audit.Emitter for end-to-end assembly tests.
// Mirrored locally (NOT cross-imported from obs/audit/middleware_test.go) to
// keep test packages independent.
type captureEmitter struct {
	events []*auditpb.AuditEvent
}

func (e *captureEmitter) Emit(_ context.Context, event *auditpb.AuditEvent) error {
	e.events = append(e.events, event)
	return nil
}

func (e *captureEmitter) Close() error { return nil }

// TestServer_FailurePath_EmitsViaOuterCollector locks in the spec correction
// (audit-context-collector capability, scenario "失败路径仍能 emit"): when
// authn short-circuits on failure, an OUTER-mounted Collector should still
// run in the LIFO post-phase and emit the AUTHN_RESULT event from the
// ctx-bound AuthnDetail.
//
// CURRENT STATUS: skipped. The implementation in obs/audit/{context,collector}.go
// uses context.WithValue, which produces a child ctx whose mutations are
// invisible to the parent ctx held by an outer middleware. With Kratos
// middleware.Chain (no ctx-merging on the way back up the stack), an outer
// Collector cannot observe a detail written by inner authn.
//
// To make this scenario implementable, obs/audit must adopt a mutable-holder
// pattern (single *holder installed once, both authn and Collector mutate /
// read its fields). That change touches obs/audit/{context.go,collector.go},
// which is out of Task 4 scope per the controller brief
// ("DO NOT touch obs/audit/collector.go ... Task 5/6 will sync"). This test
// is preserved as a forcing function for that follow-up task; flip the t.Skip
// off once the holder pattern lands.
func TestServer_FailurePath_EmitsViaOuterCollector(t *testing.T) {
	t.Skip("blocked: requires holder pattern in obs/audit/{context,collector}.go; tracked for Task 5/6")

	emitter := &captureEmitter{}
	rec := audit.NewRecorder(emitter, "test-svc")

	sentinel := errors.New("auth failed")
	failAuth := &fakeAuthenticator{returnErr: sentinel}

	// Correct mounting per spec: Collector OUTER, authn INNER.
	chain := middleware.Chain(audit.Collector(rec), Server(failAuth))
	handler := chain(func(ctx context.Context, req any) (any, error) {
		t.Fatal("inner handler must not run on authn failure")
		return nil, nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer x"})
	if _, err := handler(ctx, nil); !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel", err)
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
		t.Error("Result.Success = true on authn-failure event (should reflect AuthnDetail.Success=false)")
	}
}

// TestExtractBearerToken checks the exported helper.
func TestExtractBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"Bearer mytoken", "mytoken"},
		{"bearer mytoken", "mytoken"},
		{"BEARER mytoken", "mytoken"},
		{"Basic abc123", ""},
		{"mytoken", ""},
	}
	for _, tc := range cases {
		got := ExtractBearerToken(tc.header)
		if got != tc.want {
			t.Errorf("ExtractBearerToken(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

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

// fakeTransport implements transport.Transporter for test purposes.
// Retained because some scenarios still build a transport-bearing ctx for
// realism even though the dispatcher itself no longer reads transport.
type fakeTransport struct {
	headers map[string]string
}

func (f *fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string                { return "" }
func (f *fakeTransport) Operation() string               { return "" }
func (f *fakeTransport) RequestHeader() transport.Header { return &fakeHeader{f.headers} }
func (f *fakeTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

type fakeHeader struct {
	m map[string]string
}

func (h *fakeHeader) Get(key string) string      { return h.m[key] }
func (h *fakeHeader) Set(key, value string)      { h.m[key] = value }
func (h *fakeHeader) Add(key, value string)      {}
func (h *fakeHeader) Keys() []string             { return nil }
func (h *fakeHeader) Values(key string) []string { return nil }

// transportCtx builds a server-side ctx with a fake transport AND a fresh
// audit detail holder. The holder install mirrors what audit.Collector does
// at the chain entry in production: without it, security middleware writes
// (audit.WithAuthnResult / audit.WithAuthzResult) silently drop.
func transportCtx(headers map[string]string) context.Context {
	ctx := transport.NewServerContext(context.Background(), &fakeTransport{headers: headers})
	return audit.InstallHolder(ctx)
}

// holderCtx 返回 audit holder ctx，不挂 transport——用于 dispatcher-only 路径测试。
// dispatcher 重构后 main package 不再读 transport；新增的 dispatcher-only 测试用这个，
// 与 transportCtx 的语义区分开：用 transportCtx 的测试是"端到端 collector 管线相关"，
// 用 holderCtx 的测试是"纯 dispatcher 行为"。
func holderCtx() context.Context {
	return audit.InstallHolder(context.Background())
}

// fakeAuthenticator is a minimal Authenticator for unit tests. After the
// engine-agnostic refactor the interface contains only `Authenticate` —
// `Method()` has moved to the wrapper layer via `WithMethod` option.
type fakeAuthenticator struct {
	returnActor actor.Actor
	returnErr   error
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

// TestServer_AuthenticatorCalled checks that the authenticator is invoked and
// its returned actor is propagated into ctx for the handler.
func TestServer_AuthenticatorCalled(t *testing.T) {
	userActor := actor.NewUserActor(actor.UserActorParams{ID: "u1", DisplayName: "Test"})
	auth := &fakeAuthenticator{returnActor: userActor}
	mw := Server(auth, WithMethod("jwt"))

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

// TestServer_AuthenticatorError_Propagated checks that errors from the authenticator propagate.
func TestServer_AuthenticatorError_Propagated(t *testing.T) {
	sentinel := errors.New("auth failed")
	auth := &fakeAuthenticator{returnErr: sentinel}
	mw := Server(auth, WithMethod("jwt"))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on auth error")
		return nil, nil
	})

	ctx := transportCtx(map[string]string{})
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
	mw := Server(auth,
		WithMethod("jwt"),
		WithErrorHandler(func(_ context.Context, _ error) error { return customErr }),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on auth error")
		return nil, nil
	})

	ctx := transportCtx(map[string]string{})
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
		mw := Server(auth, WithMethod("jwt"))

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
		mw := Server(auth,
			WithMethod("jwt"),
			WithErrorHandler(func(ctx context.Context, err error) error {
				capturedCtx = ctx
				return err
			}),
		)

		handler := mw(func(ctx context.Context, req any) (any, error) {
			t.Fatal("handler should not run on auth failure")
			return nil, nil
		})

		ctx := transportCtx(map[string]string{})
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
		mw := Server(auth, WithMethod("jwt"))

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
func TestServer_FailurePath_EmitsViaOuterCollector(t *testing.T) {
	emitter := &captureEmitter{}
	rec := audit.NewRecorder(emitter, "test-svc")

	sentinel := errors.New("auth failed")
	failAuth := &fakeAuthenticator{returnErr: sentinel}

	// Correct mounting per spec: Collector OUTER, authn INNER.
	chain := middleware.Chain(audit.Collector(rec), Server(failAuth, WithMethod("jwt")))
	handler := chain(func(ctx context.Context, req any) (any, error) {
		t.Fatal("inner handler must not run on authn failure")
		return nil, nil
	})

	ctx := transportCtx(map[string]string{})
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

// ============================================================================
// New tests for the engine-agnostic refactor (Task 1).
// ============================================================================

// minimalAuthenticator implements the post-refactor `Authenticator` interface
// with ONLY one method `Authenticate`. The compile-time assertion below is the
// regression guard: if anyone re-adds `Method() string` to the interface, this
// file fails to compile.
type minimalAuthenticator struct{}

func (minimalAuthenticator) Authenticate(_ context.Context) (actor.Actor, error) {
	return actor.NewAnonymousActor(), nil
}

// Compile-time check for the single-method contract. If someone re-adds
// `Method() string` to the interface, this assertion (and the file) fails to
// compile — the cheapest possible regression guard.
var _ Authenticator = (*minimalAuthenticator)(nil)

// TestAuthenticator_SingleMethodInterface — runtime smoke that the minimal
// Authenticator (no Method()) compiles & runs through Server.
func TestAuthenticator_SingleMethodInterface(t *testing.T) {
	mw := Server(&minimalAuthenticator{}, WithMethod("minimal"))
	handler := mw(func(ctx context.Context, _ any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in ctx")
		}
		if a.Type() != actor.TypeAnonymous {
			t.Errorf("actor.Type = %v, want anonymous", a.Type())
		}
		return nil, nil
	})

	if _, err := handler(audit.InstallHolder(context.Background()), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestServer_DoesNotCallTransportFromServerContext is a structural guard: read
// the source of authn.go and assert there is no `transport.FromServerContext`
// reference. Equivalent to grepping the source file.
func TestServer_DoesNotCallTransportFromServerContext(t *testing.T) {
	src, err := os.ReadFile("authn.go")
	if err != nil {
		t.Fatalf("read authn.go: %v", err)
	}
	body := string(src)
	if strings.Contains(body, "transport.FromServerContext") {
		t.Error("authn.go MUST NOT reference transport.FromServerContext after refactor")
	}
	if strings.Contains(body, "ExtractBearerToken") {
		t.Error("authn.go MUST NOT reference ExtractBearerToken after refactor (moved to jwt sub-package as private)")
	}
	if strings.Contains(body, "svrmw") {
		t.Error("authn.go MUST NOT import or reference svrmw (transport server middleware) after refactor")
	}
	if strings.Contains(body, "authenticator.Method()") {
		t.Error("authn.go MUST NOT call authenticator.Method() after refactor; method comes from WithMethod option")
	}
}

// TestServer_MethodFromWithMethodOption asserts the Method field of the
// ctx-bound AuthnDetail equals the string passed to WithMethod(...).
//
// The table drives BOTH branches inside Server's dispatch:
//   - success branch (authenticator returns an actor, nil error)
//   - failure branch (authenticator returns an error; ctx is still written
//     before returning so an outer Collector can emit)
//
// Empty-string method is also covered: the framework main package is
// agnostic to the string; missing/empty must NOT crash and must be written
// verbatim into AuthnDetail.Method (no silent "default to jwt" fallback).
func TestServer_MethodFromWithMethodOption(t *testing.T) {
	failure := errors.New("test failure")

	cases := []struct {
		name    string
		method  string
		withErr bool // when true, fakeAuthenticator returns `failure`
	}{
		{"jwt-success", "jwt", false},
		{"jwt-failure", "jwt", true},
		{"mtls-success", "mtls", false},
		{"passkey-success", "passkey", false},
		{"custom-failure", "custom-engine", true},
		{"empty-success", "", false},
		{"empty-failure", "", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var auth *fakeAuthenticator
			if tc.withErr {
				auth = &fakeAuthenticator{returnErr: failure}
			} else {
				auth = &fakeAuthenticator{returnActor: actor.NewAnonymousActor()}
			}

			// capturedCtx is filled either by the inner handler (success path)
			// or by the WithErrorHandler hook (failure path — handler does not
			// run when authn fails).
			var capturedCtx context.Context
			opts := []Option{WithMethod(tc.method)}
			if tc.withErr {
				opts = append(opts, WithErrorHandler(func(ctx context.Context, err error) error {
					capturedCtx = ctx
					return err
				}))
			}
			mw := Server(auth, opts...)

			handler := mw(func(ctx context.Context, _ any) (any, error) {
				if tc.withErr {
					t.Fatal("handler should not run on auth failure")
				}
				capturedCtx = ctx
				return nil, nil
			})

			ctx := holderCtx()
			_, err := handler(ctx, nil)
			if tc.withErr {
				if !errors.Is(err, failure) {
					t.Fatalf("err = %v, want %v", err, failure)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			d, ok := audit.AuthnResultFrom(capturedCtx)
			if !ok {
				t.Fatal("expected AuthnDetail in ctx")
			}
			if d.Method != tc.method {
				t.Errorf("Method = %q, want %q (from WithMethod option)", d.Method, tc.method)
			}
			if tc.withErr {
				if d.Success {
					t.Errorf("Success = true, want false (failure branch)")
				}
				if d.FailureReason != failure.Error() {
					t.Errorf("FailureReason = %q, want %q", d.FailureReason, failure.Error())
				}
			} else {
				if !d.Success {
					t.Errorf("Success = false, want true (success branch)")
				}
				if d.FailureReason != "" {
					t.Errorf("FailureReason = %q, want empty (success branch)", d.FailureReason)
				}
			}
		})
	}
}

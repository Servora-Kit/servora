package authn

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/core/actor"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// fakeTransport implements transport.Transporter for test purposes.
type fakeTransport struct {
	headers map[string]string
}

func (f *fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string               { return "" }
func (f *fakeTransport) Operation() string              { return "" }
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

func transportCtx(headers map[string]string) context.Context {
	return transport.NewServerContext(context.Background(), &fakeTransport{headers: headers})
}

// fakeAuthenticator is a minimal Authenticator for unit tests.
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

// captureObserver returns an AuthnDetail-collecting callback plus an accessor
// returning the captured slice. Mirrors the captureEmitter pattern in
// authz_test.go / bridge_test.go.
func captureObserver() (func(context.Context, AuthnDetail), func() []AuthnDetail) {
	var captured []AuthnDetail
	fn := func(_ context.Context, d AuthnDetail) {
		captured = append(captured, d)
	}
	get := func() []AuthnDetail { return captured }
	return fn, get
}

// TestServer_Observer_SuccessTriggers asserts the observer fires once on a
// successful authentication, with the authenticated actor and Allowed=true.
func TestServer_Observer_SuccessTriggers(t *testing.T) {
	userActor := actor.NewUserActor(actor.UserActorParams{ID: "u1", DisplayName: "Test"})
	auth := &fakeAuthenticator{returnActor: userActor}
	obsFn, get := captureObserver()
	mw := Server(auth, WithObserver(obsFn))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := get()
	if len(got) != 1 {
		t.Fatalf("observer call count = %d, want 1", len(got))
	}
	d := got[0]
	if d.Method != "jwt" {
		t.Errorf("Method = %q, want jwt", d.Method)
	}
	if d.Subject == nil || d.Subject.ID() != "u1" {
		t.Errorf("Subject.ID = %v, want u1", d.Subject)
	}
	if !d.Allowed {
		t.Errorf("Allowed = false, want true")
	}
	if d.Err != nil {
		t.Errorf("Err = %v, want nil", d.Err)
	}
}

// TestServer_Observer_FailureTriggers asserts the observer fires on the
// default error-return path (no errorHandler), with anonymous subject,
// Allowed=false, and the original error preserved.
func TestServer_Observer_FailureTriggers(t *testing.T) {
	sentinel := errors.New("auth failed")
	auth := &fakeAuthenticator{returnErr: sentinel}
	obsFn, get := captureObserver()
	mw := Server(auth, WithObserver(obsFn))

	handlerCalled := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
	_, err := handler(ctx, nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel", err)
	}
	if handlerCalled {
		t.Error("handler should not be called on auth failure")
	}

	got := get()
	if len(got) != 1 {
		t.Fatalf("observer call count = %d, want 1", len(got))
	}
	d := got[0]
	if d.Subject == nil || d.Subject.Type() != actor.TypeAnonymous {
		t.Errorf("Subject.Type = %v, want TypeAnonymous", d.Subject)
	}
	if d.Allowed {
		t.Errorf("Allowed = true, want false")
	}
	if !errors.Is(d.Err, sentinel) {
		t.Errorf("detail.Err = %v, want sentinel", d.Err)
	}
}

// TestServer_Observer_NilSafe asserts that omitting WithObserver leaves the
// middleware unaffected (no panic, handler still runs).
func TestServer_Observer_NilSafe(t *testing.T) {
	userActor := actor.NewUserActor(actor.UserActorParams{ID: "u1"})
	auth := &fakeAuthenticator{returnActor: userActor}
	mw := Server(auth) // no WithObserver

	handlerCalled := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	})

	ctx := transportCtx(map[string]string{"Authorization": "Bearer sometoken"})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler should be called when no observer is configured")
	}
}

// TestServer_Observer_AnonymousSuccess asserts the observer fires with
// Allowed=true and TypeAnonymous when the authenticator returns an anonymous
// actor (e.g. JWT engine sees no Authorization header).
func TestServer_Observer_AnonymousSuccess(t *testing.T) {
	auth := &fakeAuthenticator{returnActor: actor.NewAnonymousActor()}
	obsFn, get := captureObserver()
	mw := Server(auth, WithObserver(obsFn))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})

	ctx := transportCtx(map[string]string{}) // no Authorization header
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := get()
	if len(got) != 1 {
		t.Fatalf("observer call count = %d, want 1", len(got))
	}
	d := got[0]
	if d.Subject == nil || d.Subject.Type() != actor.TypeAnonymous {
		t.Errorf("Subject.Type = %v, want TypeAnonymous", d.Subject)
	}
	if !d.Allowed {
		t.Errorf("Allowed = false, want true")
	}
	if d.Err != nil {
		t.Errorf("Err = %v, want nil", d.Err)
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

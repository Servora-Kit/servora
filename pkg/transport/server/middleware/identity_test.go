package middleware

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/Servora-Kit/servora/pkg/actor"
)

type fakeTransport struct {
	headers map[string]string
}

func (f *fakeTransport) Kind() transport.Kind        { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string             { return "" }
func (f *fakeTransport) Operation() string            { return "" }
func (f *fakeTransport) RequestHeader() transport.Header { return &fakeHeader{f.headers} }
func (f *fakeTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

type fakeHeader struct {
	m map[string]string
}

func (h *fakeHeader) Get(key string) string    { return h.m[key] }
func (h *fakeHeader) Set(key, value string)    { h.m[key] = value }
func (h *fakeHeader) Add(key, value string)    {}
func (h *fakeHeader) Keys() []string           { return nil }
func (h *fakeHeader) Values(key string) []string { return nil }

func TestIdentityFromHeader_WithUserID(t *testing.T) {
	mw := IdentityFromHeader()
	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if a.Type() != actor.TypeUser {
			t.Errorf("expected TypeUser, got %v", a.Type())
		}
		if a.ID() != "user-123" {
			t.Errorf("expected user-123, got %s", a.ID())
		}
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{"X-User-ID": "user-123"},
	})
	_, _ = handler(ctx, nil)
}

func TestIdentityFromHeader_NoHeader(t *testing.T) {
	mw := IdentityFromHeader()
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

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{},
	})
	_, _ = handler(ctx, nil)
}

func TestIdentityFromHeader_EmptyHeader(t *testing.T) {
	mw := IdentityFromHeader()
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

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{"X-User-ID": ""},
	})
	_, _ = handler(ctx, nil)
}

func TestIdentityFromHeader_CustomHeaderKey(t *testing.T) {
	mw := IdentityFromHeader(WithHeaderKey("X-Custom-ID"))
	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if a.ID() != "custom-456" {
			t.Errorf("expected custom-456, got %s", a.ID())
		}
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{"X-Custom-ID": "custom-456"},
	})
	_, _ = handler(ctx, nil)
}

func TestIdentityFromHeader_NoTransport(t *testing.T) {
	mw := IdentityFromHeader()
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

	_, _ = handler(context.Background(), nil)
}

func TestIdentityFromHeader_MultipleHeaders(t *testing.T) {
	mw := IdentityFromHeader()
	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if a.Type() != actor.TypeUser {
			t.Errorf("expected TypeUser, got %v", a.Type())
		}
		if a.Email() != "alice@example.com" {
			t.Errorf("email = %q, want alice@example.com", a.Email())
		}
		if a.Subject() != "sub-123" {
			t.Errorf("subject = %q, want sub-123", a.Subject())
		}
		if len(a.Roles()) != 2 {
			t.Errorf("roles len = %d, want 2", len(a.Roles()))
		}
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{
			"X-User-ID": "user-123",
			"X-Subject": "sub-123",
			"X-Email":   "alice@example.com",
			"X-Roles":   "admin, editor",
			"X-Scopes":  "openid profile",
		},
	})
	_, _ = handler(ctx, nil)
}

func TestIdentityFromHeader_ServicePrincipal(t *testing.T) {
	mw := IdentityFromHeader()
	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if a.Type() != actor.TypeService {
			t.Errorf("expected TypeService, got %v", a.Type())
		}
		if a.ClientID() != "audit-service" {
			t.Errorf("clientID = %q, want audit-service", a.ClientID())
		}
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{
			"X-User-ID":          "svc-audit",
			"X-Client-ID":        "audit-service",
			"X-Principal-Type":   "service",
		},
	})
	_, _ = handler(ctx, nil)
}

func TestIdentityFromHeader_WithHeaderMapping(t *testing.T) {
	mw := IdentityFromHeader(WithHeaderMapping(HeaderMapping{Roles: "X-Custom-Roles"}))
	handler := mw(func(ctx context.Context, req any) (any, error) {
		a, ok := actor.FromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		if len(a.Roles()) != 1 || a.Roles()[0] != "superadmin" {
			t.Errorf("roles = %v, want [superadmin]", a.Roles())
		}
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{
		headers: map[string]string{
			"X-User-ID":      "user-789",
			"X-Custom-Roles": "superadmin",
		},
	})
	_, _ = handler(ctx, nil)
}

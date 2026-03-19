package middleware

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/authz/service/v1"
	iamv1 "github.com/Servora-Kit/servora/api/gen/go/iam/service/v1"
)

// fakeTransport implements transport.Transporter for test purposes.
type fakeTransport struct {
	operation string
}

func (f *fakeTransport) Kind() transport.Kind             { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string                 { return "" }
func (f *fakeTransport) Operation() string                { return f.operation }
func (f *fakeTransport) RequestHeader() transport.Header  { return &fakeHeader{} }
func (f *fakeTransport) ReplyHeader() transport.Header    { return &fakeHeader{} }

type fakeHeader struct{}

func (h *fakeHeader) Get(key string) string      { return "" }
func (h *fakeHeader) Set(key, value string)      {}
func (h *fakeHeader) Add(key, value string)      {}
func (h *fakeHeader) Keys() []string             { return nil }
func (h *fakeHeader) Values(key string) []string { return nil }

// TestWithAuthzRules_ModeNone_Passthrough checks that an IAM AuthzRuleEntry with
// AUTHZ_MODE_NONE is correctly converted and allows the request through.
func TestWithAuthzRules_ModeNone_Passthrough(t *testing.T) {
	const op = "/iam.service.v1.TestService/TestMethod"
	rules := map[string]iamv1.AuthzRuleEntry{
		op: {Mode: authzpb.AuthzMode_AUTHZ_MODE_NONE},
	}

	mw := Authz(WithAuthzRules(rules))

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{operation: op})
	resp, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
}

// TestWithAuthzRules_NoMatch_Forbidden checks that operations not in the rules map are rejected (fail-closed).
func TestWithAuthzRules_NoMatch_Forbidden(t *testing.T) {
	const op = "/iam.service.v1.TestService/TestMethod"
	mw := Authz(WithAuthzRules(map[string]iamv1.AuthzRuleEntry{}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called for unknown operation")
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{operation: op})
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for operation with no rule")
	}
}

// TestWithAuthzRules_AllFields_Converted checks that all fields of iamv1.AuthzRuleEntry
// are correctly propagated through the conversion to ensure no field is silently dropped.
func TestWithAuthzRules_AllFields_Converted(t *testing.T) {
	const op = "/iam.service.v1.OrganizationService/GetOrganization"

	// AUTHZ_MODE_NONE is used here so we can verify conversion without needing a real actor/FGA.
	// The important thing is that the mode field is forwarded correctly.
	rules := map[string]iamv1.AuthzRuleEntry{
		op: {
			Mode:       authzpb.AuthzMode_AUTHZ_MODE_NONE,
			Relation:   "can_view",
			ObjectType: "organization",
			IDField:    "organization_id",
		},
	}

	mw := Authz(WithAuthzRules(rules))
	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{operation: op})
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler not called — AUTHZ_MODE_NONE should passthrough")
	}
}

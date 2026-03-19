package authz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/authz/service/v1"
	userpb "github.com/Servora-Kit/servora/api/gen/go/user/service/v1"
	"github.com/Servora-Kit/servora/pkg/actor"
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

func transportCtx(operation string) context.Context {
	return transport.NewServerContext(context.Background(), &fakeTransport{operation: operation})
}

func userActorCtx(ctx context.Context, userID string) context.Context {
	return actor.NewContext(ctx, actor.NewUserActor(userID, "Test User", "test@example.com", nil))
}

const testOp = "/test.service.v1.TestService/TestMethod"

// TestAuthz_NoRule_Forbidden checks that operations with no rule are rejected (fail-closed).
func TestAuthz_NoRule_Forbidden(t *testing.T) {
	mw := Authz() // no rules configured

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called when no rule exists")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for missing rule")
	}
}

// TestAuthz_ModeNone_Passthrough checks that AUTHZ_MODE_NONE skips authorization.
func TestAuthz_ModeNone_Passthrough(t *testing.T) {
	mw := Authz(WithAuthzRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_NONE},
	}))

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := transportCtx(testOp)
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

// TestAuthz_CheckMode_AnonymousActor_Forbidden checks that anonymous actors are denied.
func TestAuthz_CheckMode_AnonymousActor_Forbidden(t *testing.T) {
	mw := Authz(WithAuthzRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called for anonymous actor")
		return nil, nil
	})

	// No actor in context (anonymous)
	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for anonymous actor")
	}
}

// TestAuthz_CheckMode_NoActor_Forbidden checks that missing actor is denied.
func TestAuthz_CheckMode_NoActor_Forbidden(t *testing.T) {
	mw := Authz(WithAuthzRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called with no actor")
		return nil, nil
	})

	// Actor exists but is anonymous type
	ctx := transport.NewServerContext(context.Background(), &fakeTransport{operation: testOp})
	ctx = actor.NewContext(ctx, &anonymousActor{})
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for anonymous-type actor")
	}
}

// TestAuthz_CheckMode_NilFGA_ServiceUnavailable checks that nil FGA client returns 503.
func TestAuthz_CheckMode_NilFGA_ServiceUnavailable(t *testing.T) {
	mw := Authz(
		WithAuthzRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
		}),
		// no WithFGAClient → fga is nil
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called with nil FGA client")
		return nil, nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil FGA client")
	}
}

// TestAuthz_NoTransport_Passthrough checks that requests without server transport are passed through.
func TestAuthz_NoTransport_Passthrough(t *testing.T) {
	mw := Authz() // no rules needed — no transport means skip

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	// Plain background context, no transport
	_, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

// TestAuthz_IDField_Empty_UsesDefault checks that an empty IDField results in "default" object ID.
func TestAuthz_IDField_Empty_UsesDefault(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "platform", IDField: ""}
	objectType, objectID, err := resolveObject(rule, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectType != "platform" {
		t.Errorf("objectType = %q, want platform", objectType)
	}
	if objectID != "default" {
		t.Errorf("objectID = %q, want default", objectID)
	}
}

// TestAuthz_IDField_Set_ExtractedFromProto checks that IDField is extracted from the proto request.
func TestAuthz_IDField_Set_ExtractedFromProto(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "user", IDField: "id"}
	req := &userpb.GetUserRequest{Id: "user-abc-123"}

	objectType, objectID, err := resolveObject(rule, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectType != "user" {
		t.Errorf("objectType = %q, want user", objectType)
	}
	if objectID != "user-abc-123" {
		t.Errorf("objectID = %q, want user-abc-123", objectID)
	}
}

// TestAuthz_IDField_NotFound_Error checks that a missing proto field returns an error.
func TestAuthz_IDField_NotFound_Error(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "user", IDField: "nonexistent_field"}
	req := &userpb.GetUserRequest{Id: "user-abc-123"}

	_, _, err := resolveObject(rule, req)
	if err == nil {
		t.Fatal("expected error for nonexistent field")
	}
}

// TestAuthz_ObjectType_Empty_Error checks that an empty ObjectType in the rule returns an error.
func TestAuthz_ObjectType_Empty_Error(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: ""}
	_, _, err := resolveObject(rule, nil)
	if err == nil {
		t.Fatal("expected error for empty ObjectType")
	}
}

// TestExtractProtoField_NonProtoRequest_Error checks that non-proto requests return an error.
func TestExtractProtoField_NonProtoRequest_Error(t *testing.T) {
	_, err := extractProtoField("not a proto message", "id")
	if err == nil {
		t.Fatal("expected error for non-proto request")
	}
}

// TestExtractProtoField_EmptyFieldValue_Error checks that an empty field value returns an error.
func TestExtractProtoField_EmptyFieldValue_Error(t *testing.T) {
	req := &userpb.GetUserRequest{Id: ""} // empty ID
	_, err := extractProtoField(req, "id")
	if err == nil {
		t.Fatal("expected error for empty field value")
	}
}

// anonymousActor is a test actor with TypeAnonymous.
type anonymousActor struct{}

func (a *anonymousActor) ID() string          { return "" }
func (a *anonymousActor) Type() actor.Type    { return actor.TypeAnonymous }
func (a *anonymousActor) DisplayName() string { return "anonymous" }
func (a *anonymousActor) Scope(key string) string { return "" }

package authz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

	"google.golang.org/protobuf/types/known/wrapperspb"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"github.com/Servora-Kit/servora/core/actor"
)

// fakeTransport implements transport.Transporter for test purposes.
type fakeTransport struct {
	operation string
}

func (f *fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string               { return "" }
func (f *fakeTransport) Operation() string              { return f.operation }
func (f *fakeTransport) RequestHeader() transport.Header { return &fakeHeader{} }
func (f *fakeTransport) ReplyHeader() transport.Header   { return &fakeHeader{} }

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
	return actor.NewContext(ctx, actor.NewUserActor(actor.UserActorParams{ID: userID, DisplayName: "Test User", Email: "test@example.com"}))
}

const testOp = "/test.service.v1.TestService/TestMethod"

// fakeAuthorizer is a minimal Authorizer for unit tests.
type fakeAuthorizer struct {
	allowed bool
	err     error
}

func (f *fakeAuthorizer) IsAuthorized(_ context.Context, _, _, _, _ string) (bool, error) {
	return f.allowed, f.err
}

// TestServer_NoRule_Forbidden checks that operations with no rule are rejected (fail-closed).
func TestServer_NoRule_Forbidden(t *testing.T) {
	mw := Server(nil) // no rules configured

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

// TestServer_ModeNone_Passthrough checks that AUTHZ_MODE_NONE skips authorization.
func TestServer_ModeNone_Passthrough(t *testing.T) {
	mw := Server(nil, WithRules(map[string]AuthzRule{
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

// TestServer_CheckMode_AnonymousActor_Forbidden checks that anonymous actors are denied.
func TestServer_CheckMode_AnonymousActor_Forbidden(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: true}, WithRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called for anonymous actor")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for anonymous actor")
	}
}

// TestServer_CheckMode_NoActor_Forbidden checks that an anonymous-type actor is denied.
func TestServer_CheckMode_NoActor_Forbidden(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: true}, WithRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called with anonymous-type actor")
		return nil, nil
	})

	ctx := transport.NewServerContext(context.Background(), &fakeTransport{operation: testOp})
	ctx = actor.NewContext(ctx, &anonymousActor{})
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for anonymous-type actor")
	}
}

// TestServer_CheckMode_NilAuthorizer_ServiceUnavailable checks that nil authorizer returns 503.
func TestServer_CheckMode_NilAuthorizer_ServiceUnavailable(t *testing.T) {
	mw := Server(nil, WithRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called with nil authorizer")
		return nil, nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil authorizer")
	}
}

// TestServer_CheckMode_Allowed checks that an allowed subject passes through.
func TestServer_CheckMode_Allowed(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: true}, WithRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

// TestServer_CheckMode_Denied checks that a denied subject gets 403.
func TestServer_CheckMode_Denied(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: false}, WithRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called for denied subject")
		return nil, nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for denied subject")
	}
}

// TestServer_NoTransport_Passthrough checks that requests without server transport are passed through.
func TestServer_NoTransport_Passthrough(t *testing.T) {
	mw := Server(nil) // no rules needed — no transport means skip

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	_, err := handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

// TestServer_DecisionLogger_Called checks that WithDecisionLogger is invoked.
func TestServer_DecisionLogger_Called(t *testing.T) {
	var logged *DecisionDetail
	mw := Server(
		&fakeAuthorizer{allowed: true},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
		}),
		WithDecisionLogger(func(_ context.Context, d DecisionDetail) {
			logged = &d
		}),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) { return nil, nil })
	ctx := userActorCtx(transportCtx(testOp), "user-123")
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logged == nil {
		t.Fatal("expected DecisionLogger to be called")
	}
	if !logged.Allowed {
		t.Errorf("logged.Allowed = false, want true")
	}
	if logged.Operation != testOp {
		t.Errorf("logged.Operation = %q, want %q", logged.Operation, testOp)
	}
}

// TestResolveObject_IDField_Empty_UsesDefault checks that an empty IDField results in "default".
func TestResolveObject_IDField_Empty_UsesDefault(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "platform", IDField: ""}
	objectType, objectID, err := resolveObject(rule, nil, "default")
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

// TestResolveObject_IDField_Empty_CustomDefault checks WithDefaultObjectID option.
func TestResolveObject_IDField_Empty_CustomDefault(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "platform", IDField: ""}
	objectType, objectID, err := resolveObject(rule, nil, "global")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectType != "platform" {
		t.Errorf("objectType = %q, want platform", objectType)
	}
	if objectID != "global" {
		t.Errorf("objectID = %q, want global", objectID)
	}
}

// TestResolveObject_IDField_Set_ExtractedFromProto checks that IDField is extracted from the proto request.
func TestResolveObject_IDField_Set_ExtractedFromProto(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "user", IDField: "value"}
	req := &wrapperspb.StringValue{Value: "user-abc-123"}

	objectType, objectID, err := resolveObject(rule, req, "default")
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

// TestResolveObject_IDField_NotFound_Error checks that a missing proto field returns an error.
func TestResolveObject_IDField_NotFound_Error(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: "user", IDField: "nonexistent_field"}
	req := &wrapperspb.StringValue{Value: "user-abc-123"}

	_, _, err := resolveObject(rule, req, "default")
	if err == nil {
		t.Fatal("expected error for nonexistent field")
	}
}

// TestResolveObject_ObjectType_Empty_Error checks that an empty ObjectType returns an error.
func TestResolveObject_ObjectType_Empty_Error(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ObjectType: ""}
	_, _, err := resolveObject(rule, nil, "default")
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
	req := &wrapperspb.StringValue{Value: ""} // empty value
	_, err := extractProtoField(req, "value")
	if err == nil {
		t.Fatal("expected error for empty field value")
	}
}

// anonymousActor is a test actor with TypeAnonymous.
type anonymousActor struct{}

func (a *anonymousActor) ID() string               { return "" }
func (a *anonymousActor) Type() actor.Type         { return actor.TypeAnonymous }
func (a *anonymousActor) DisplayName() string      { return "anonymous" }
func (a *anonymousActor) Email() string            { return "" }
func (a *anonymousActor) Subject() string          { return "" }
func (a *anonymousActor) ClientID() string         { return "" }
func (a *anonymousActor) Realm() string            { return "" }
func (a *anonymousActor) Roles() []string          { return []string{} }
func (a *anonymousActor) Scopes() []string         { return []string{} }
func (a *anonymousActor) Attrs() map[string]string { return map[string]string{} }
func (a *anonymousActor) Scope(_ string) string    { return "" }

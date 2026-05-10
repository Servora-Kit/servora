package authz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"google.golang.org/protobuf/types/known/wrapperspb"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/obs/audit"
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

// transportCtx builds a server-side ctx with a fake transport AND a fresh
// audit detail holder. The holder install mirrors what audit.Collector does
// at the chain entry in production: without it, security middleware writes
// (audit.WithAuthzResult) silently drop. Tests that assert ctx-bound details
// after the middleware runs require the holder to be present up-front.
func transportCtx(operation string) context.Context {
	ctx := transport.NewServerContext(context.Background(), &fakeTransport{operation: operation})
	return audit.InstallHolder(ctx)
}

func userActorCtx(ctx context.Context, userID string) context.Context {
	return actor.NewContext(ctx, actor.NewUserActor(userID, "Test User"))
}

const testOp = "/test.service.v1.TestService/TestMethod"

// fakeAuthorizer is a minimal Authorizer for unit tests.
type fakeAuthorizer struct {
	allowed        bool
	err            error
	listAllowedIDs []string
}

func (f *fakeAuthorizer) Check(_ context.Context, _, _, _, _ string) (bool, error) {
	return f.allowed, f.err
}

func (f *fakeAuthorizer) BatchCheck(_ context.Context, reqs []CheckRequest) ([]CheckResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]CheckResult, len(reqs))
	for i := range reqs {
		out[i] = CheckResult{Allowed: f.allowed}
	}
	return out, nil
}

func (f *fakeAuthorizer) ListAllowed(_ context.Context, _, _, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listAllowedIDs, nil
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

// TestServer_WritesAuthzResultToContext exercises the three-state Decision
// mapping (allowed / denied / error) and asserts the *auditpb.AuthzDetail
// written to ctx via audit.WithAuthzResult matches expectations.
//
// Pipeline contract: middleware writes detail BEFORE returning (even on
// denial / authorizer error), so an outer-mounted audit.Collector can read
// it post-handler and emit an AUTHZ_DECISION event.
//
// Reading via the OUTER ctx works thanks to the holder pattern: holder is
// a pointer threaded through the original ctx, so inner mutations propagate
// upward despite Go's context.WithValue immutability.
func TestServer_WritesAuthzResultToContext(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"}

	t.Run("allowed", func(t *testing.T) {
		mw := Server(&fakeAuthorizer{allowed: true}, WithRules(map[string]AuthzRule{testOp: rule}))
		handler := mw(func(ctx context.Context, req any) (any, error) { return nil, nil })

		ctx := userActorCtx(transportCtx(testOp), "user-123")
		if _, err := handler(ctx, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		d, ok := audit.AuthzResultFrom(ctx)
		if !ok {
			t.Fatal("expected AuthzDetail in ctx")
		}
		if d.Decision != auditpb.AuthzDecision_AUTHZ_DECISION_ALLOWED {
			t.Errorf("Decision = %v, want ALLOWED", d.Decision)
		}
		if d.ErrorReason != "" {
			t.Errorf("ErrorReason = %q, want empty", d.ErrorReason)
		}
		if d.Relation != "admin" {
			t.Errorf("Relation = %q, want admin", d.Relation)
		}
		if d.ObjectType != "platform" {
			t.Errorf("ObjectType = %q, want platform", d.ObjectType)
		}
		if d.ObjectId != "default" {
			t.Errorf("ObjectId = %q, want default", d.ObjectId)
		}
	})

	t.Run("denied", func(t *testing.T) {
		mw := Server(&fakeAuthorizer{allowed: false}, WithRules(map[string]AuthzRule{testOp: rule}))
		handler := mw(func(ctx context.Context, req any) (any, error) {
			t.Fatal("handler should not run on deny")
			return nil, nil
		})

		ctx := userActorCtx(transportCtx(testOp), "user-123")
		_, err := handler(ctx, nil)
		if err == nil {
			t.Fatal("expected denied error")
		}

		d, ok := audit.AuthzResultFrom(ctx)
		if !ok {
			t.Fatal("expected AuthzDetail in ctx (written before deny return)")
		}
		if d.Decision != auditpb.AuthzDecision_AUTHZ_DECISION_DENIED {
			t.Errorf("Decision = %v, want DENIED", d.Decision)
		}
		if d.ErrorReason != "" {
			t.Errorf("ErrorReason = %q, want empty on deny", d.ErrorReason)
		}
	})

	t.Run("error", func(t *testing.T) {
		sentinel := errors.New("backend down")
		mw := Server(&fakeAuthorizer{err: sentinel}, WithRules(map[string]AuthzRule{testOp: rule}))
		handler := mw(func(ctx context.Context, req any) (any, error) {
			t.Fatal("handler should not run on authorizer error")
			return nil, nil
		})

		ctx := userActorCtx(transportCtx(testOp), "user-123")
		_, err := handler(ctx, nil)
		if err == nil {
			t.Fatal("expected error")
		}

		d, ok := audit.AuthzResultFrom(ctx)
		if !ok {
			t.Fatal("expected AuthzDetail in ctx (written before error return)")
		}
		if d.Decision != auditpb.AuthzDecision_AUTHZ_DECISION_ERROR {
			t.Errorf("Decision = %v, want ERROR", d.Decision)
		}
		if d.ErrorReason != sentinel.Error() {
			t.Errorf("ErrorReason = %q, want %q", d.ErrorReason, sentinel.Error())
		}
	})
}

// captureEmitter is a minimal audit.Emitter for end-to-end assembly tests.
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
// authz short-circuits on denial, an OUTER-mounted Collector should still
// run in the LIFO post-phase and emit the AUTHZ_DECISION event from the
// ctx-bound AuthzDetail.
//
// Mirrors security/authn/authn_test.go:TestServer_FailurePath_EmitsViaOuterCollector
// for the authz half of the push-ctx pipeline.
func TestServer_FailurePath_EmitsViaOuterCollector(t *testing.T) {
	emitter := &captureEmitter{}
	rec := audit.NewRecorder(emitter, "test-svc")

	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"}

	// Correct mounting per spec: Collector OUTER, authz INNER.
	chain := middleware.Chain(
		audit.Collector(rec),
		Server(&fakeAuthorizer{allowed: false}, WithRules(map[string]AuthzRule{testOp: rule})),
	)
	handler := chain(func(ctx context.Context, req any) (any, error) {
		t.Fatal("inner handler must not run on authz denial")
		return nil, nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected denial error")
	}

	if len(emitter.events) != 1 {
		t.Fatalf("emit count = %d, want 1", len(emitter.events))
	}
	evt := emitter.events[0]
	if evt.GetEventType() != auditpb.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION {
		t.Errorf("EventType = %v, want AUTHZ_DECISION", evt.GetEventType())
	}
	d := evt.GetAuthzDetail()
	if d == nil {
		t.Fatal("AuthzDetail missing in emitted event")
	}
	if d.GetDecision() != auditpb.AuthzDecision_AUTHZ_DECISION_DENIED {
		t.Errorf("AuthzDetail.Decision = %v, want DENIED", d.GetDecision())
	}
	if evt.GetResult().GetSuccess() {
		t.Error("Result.Success = true on authz-denial event (should reflect Decision != ALLOWED)")
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

// TestFakeAuthorizer_ImplementsBatchCheck ensures the test fake covers BatchCheck.
func TestFakeAuthorizer_ImplementsBatchCheck(t *testing.T) {
	a := &fakeAuthorizer{allowed: true}
	results, err := a.BatchCheck(context.Background(), []CheckRequest{
		{Subject: "user:alice", Relation: "viewer", ObjectType: "doc", ObjectID: "1"},
		{Subject: "user:alice", Relation: "viewer", ObjectType: "doc", ObjectID: "2"},
	})
	if err != nil {
		t.Fatalf("BatchCheck err = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if !results[0].Allowed || !results[1].Allowed {
		t.Errorf("results = %+v, want all allowed", results)
	}
}

// TestFakeAuthorizer_ImplementsListAllowed ensures the test fake covers ListAllowed.
func TestFakeAuthorizer_ImplementsListAllowed(t *testing.T) {
	a := &fakeAuthorizer{listAllowedIDs: []string{"doc:1", "doc:5"}}
	ids, err := a.ListAllowed(context.Background(), "user:alice", "viewer", "doc")
	if err != nil {
		t.Fatalf("ListAllowed err = %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("len(ids) = %d, want 2", len(ids))
	}
}

// blockingAuthorizer simulates a slow backend by waiting until ctx is cancelled.
type blockingAuthorizer struct{}

func (b *blockingAuthorizer) Check(ctx context.Context, _, _, _, _ string) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}
func (b *blockingAuthorizer) BatchCheck(ctx context.Context, _ []CheckRequest) ([]CheckResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (b *blockingAuthorizer) ListAllowed(ctx context.Context, _, _, _ string) ([]string, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestServer_CheckTimeout_TripsCheckBeforeBackend ensures Check is bounded.
func TestServer_CheckTimeout_TripsCheckBeforeBackend(t *testing.T) {
	mw := Server(
		&blockingAuthorizer{},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Relation: "admin", ObjectType: "platform"},
		}),
		WithCheckTimeout(50*time.Millisecond),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler must not be reached when check times out")
		return nil, nil
	})

	ctx := userActorCtx(transportCtx(testOp), "user-123")
	start := time.Now()
	_, err := handler(ctx, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed >= 500*time.Millisecond {
		t.Errorf("elapsed = %v, expected < 500ms (timeout should trip well before)", elapsed)
	}
}

// TestServer_FailOpenOnMissingRule_PassesThroughAndAlerts verifies the option.
func TestServer_FailOpenOnMissingRule_PassesThroughAndAlerts(t *testing.T) {
	var alerted *string
	mw := Server(nil,
		// no rules
		WithFailOpenOnMissingRule(func(ctx context.Context, operation string) {
			alerted = &operation
		}),
	)

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := transportCtx(testOp)
	resp, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("expected pass-through, got err=%v", err)
	}
	if !called {
		t.Fatal("handler must be called when fail-open is on")
	}
	if resp != "ok" {
		t.Errorf("resp = %v, want ok", resp)
	}
	if alerted == nil || *alerted != testOp {
		t.Errorf("alert callback not invoked with operation %q (got %v)", testOp, alerted)
	}
}

// TestServer_NoFailOpen_StillFailsClosed ensures default behavior is unchanged.
func TestServer_NoFailOpen_StillFailsClosed(t *testing.T) {
	mw := Server(nil) // no rules, no fail-open option
	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler must not be called by default")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected fail-closed error for missing rule")
	}
}

// TestExtractProtoField_DotPath_NestedScalar resolves a nested scalar via path.
//
// Fixture choice rationale: structpb.Struct.fields is a map<string, Value> and
// is therefore rejected at the first segment (maps are not navigable per the
// id_field contract). To genuinely exercise message→scalar traversal we use
// auditpb.AuditEvent, whose `target` field is a singular *AuditTarget message
// containing scalar fields like `id`. The path "target.id" is exactly the kind
// of `parent.id` shape this feature is designed to support.
func TestExtractProtoField_DotPath_NestedScalar(t *testing.T) {
	req := &auditpb.AuditEvent{
		Target: &auditpb.AuditTarget{Id: "outer-123"},
	}

	got, err := extractProtoField(req, "target.id")
	if err != nil {
		t.Fatalf("extractProtoField err = %v", err)
	}
	if got != "outer-123" {
		t.Errorf("got %q, want outer-123", got)
	}
}

// TestExtractProtoField_DotPath_MissingSegment errors out cleanly when a
// non-leaf segment refers to a field that does not exist on the message.
func TestExtractProtoField_DotPath_MissingSegment(t *testing.T) {
	req := &auditpb.AuditEvent{
		Target: &auditpb.AuditTarget{Id: "x"},
	}
	_, err := extractProtoField(req, "target.missing")
	if err == nil {
		t.Fatal("expected error for missing nested segment")
	}
}

// TestExtractProtoField_DotPath_TerminatesOnMessage_Errors guards against
// silently String()-ifying a message into textproto garbage.
func TestExtractProtoField_DotPath_TerminatesOnMessage_Errors(t *testing.T) {
	req := &auditpb.AuditEvent{
		Target: &auditpb.AuditTarget{Id: "x"},
	}
	// "target" alone terminates on *AuditTarget (a message), which must error.
	_, err := extractProtoField(req, "target")
	if err == nil {
		t.Fatal("expected error when path terminus is a message, not a scalar")
	}
}

// TestExtractProtoField_TopLevel_StillWorks ensures backwards compatibility
// for the existing single-segment case.
func TestExtractProtoField_TopLevel_StillWorks(t *testing.T) {
	req := &wrapperspb.StringValue{Value: "user-abc"}
	got, err := extractProtoField(req, "value")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "user-abc" {
		t.Errorf("got %q, want user-abc", got)
	}
}

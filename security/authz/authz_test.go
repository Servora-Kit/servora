package authz

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/types/known/wrapperspb"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	auditpb "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// fakeTransport implements transport.Transporter for test purposes.
type fakeTransport struct {
	operation string
}

func (f *fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (f *fakeTransport) Endpoint() string                { return "" }
func (f *fakeTransport) Operation() string               { return f.operation }
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

func subjectCtx(ctx context.Context) context.Context {
	return ctx // subject is resolved via WithSubjectFunc option, not ctx
}

const testOp = "/test.service.v1.TestService/TestMethod"

func staticSubjectFunc(subject string) func(context.Context) (string, bool) {
	return func(_ context.Context) (string, bool) {
		if subject == "" {
			return "", false
		}
		return subject, true
	}
}

// fakeAuthorizer is a minimal Authorizer for unit tests.
type fakeAuthorizer struct {
	allowed bool
	err     error
	// captured records the last CheckRequest for inspection.
	mu       sync.Mutex
	captured *CheckRequest
}

func (f *fakeAuthorizer) Check(_ context.Context, req CheckRequest) (bool, error) {
	f.mu.Lock()
	cp := req
	f.captured = &cp
	f.mu.Unlock()
	return f.allowed, f.err
}

func (f *fakeAuthorizer) lastRequest() *CheckRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.captured
}

// captureAuditor records emitted CloudEvents for assertion.
type captureAuditor struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func (a *captureAuditor) Emit(_ context.Context, event cloudevents.Event) error {
	a.mu.Lock()
	a.events = append(a.events, event)
	a.mu.Unlock()
	return nil
}

func (a *captureAuditor) getEvents() []cloudevents.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]cloudevents.Event, len(a.events))
	copy(out, a.events)
	return out
}

// ---------------------------------------------------------------------------
// Tests: Authorizer.Check interface
// ---------------------------------------------------------------------------

func TestAuthorizer_Check_WithMock(t *testing.T) {
	fa := &fakeAuthorizer{allowed: true}
	allowed, err := fa.Check(context.Background(), CheckRequest{
		Subject:      "user:alice",
		Action:       "view",
		ResourceType: "document",
		ResourceID:   "doc-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true")
	}
	req := fa.lastRequest()
	if req.Subject != "user:alice" {
		t.Errorf("Subject = %q, want user:alice", req.Subject)
	}
	if req.Action != "view" {
		t.Errorf("Action = %q, want view", req.Action)
	}
	if req.ResourceType != "document" {
		t.Errorf("ResourceType = %q, want document", req.ResourceType)
	}
	if req.ResourceID != "doc-1" {
		t.Errorf("ResourceID = %q, want doc-1", req.ResourceID)
	}
}

func TestAuthorizer_Check_Denied(t *testing.T) {
	fa := &fakeAuthorizer{allowed: false}
	allowed, err := fa.Check(context.Background(), CheckRequest{
		Subject: "user:bob", Action: "delete", ResourceType: "doc", ResourceID: "1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false")
	}
}

func TestAuthorizer_Check_Error(t *testing.T) {
	sentinel := errors.New("backend unavailable")
	fa := &fakeAuthorizer{err: sentinel}
	_, err := fa.Check(context.Background(), CheckRequest{
		Subject: "user:bob", Action: "view", ResourceType: "doc", ResourceID: "1",
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
}

// ---------------------------------------------------------------------------
// Tests: Server middleware
// ---------------------------------------------------------------------------

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

func TestServer_NoSubjectFunc_Forbidden(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: true}, WithRules(map[string]AuthzRule{
		testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
	}))

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called without SubjectFunc")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error when SubjectFunc is not set")
	}
}

func TestServer_SubjectFunc_ReturnsFalse_Forbidden(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: true},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(func(_ context.Context) (string, bool) { return "", false }),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called when SubjectFunc returns false")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error when SubjectFunc returns false")
	}
}

func TestServer_NilAuthorizer_ServiceUnavailable(t *testing.T) {
	mw := Server(nil,
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:123")),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called with nil authorizer")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil authorizer")
	}
}

func TestServer_CheckMode_Allowed(t *testing.T) {
	fa := &fakeAuthorizer{allowed: true}
	mw := Server(fa,
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:alice")),
	)

	called := false
	handler := mw(func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}

	// Verify the CheckRequest passed to authorizer.
	req := fa.lastRequest()
	if req == nil {
		t.Fatal("authorizer.Check was not called")
	}
	if req.Subject != "user:alice" {
		t.Errorf("Subject = %q, want user:alice", req.Subject)
	}
	if req.Action != "admin" {
		t.Errorf("Action = %q, want admin", req.Action)
	}
	if req.ResourceType != "platform" {
		t.Errorf("ResourceType = %q, want platform", req.ResourceType)
	}
	if req.ResourceID != "default" {
		t.Errorf("ResourceID = %q, want default", req.ResourceID)
	}
}

func TestServer_CheckMode_Denied(t *testing.T) {
	mw := Server(&fakeAuthorizer{allowed: false},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:bob")),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called for denied subject")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error for denied subject")
	}
}

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

// ---------------------------------------------------------------------------
// Tests: WithSubjectFunc
// ---------------------------------------------------------------------------

func TestServer_WithSubjectFunc_PassesSubjectToCheck(t *testing.T) {
	fa := &fakeAuthorizer{allowed: true}
	mw := Server(fa,
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "view", ResourceType: "doc"},
		}),
		WithSubjectFunc(func(_ context.Context) (string, bool) {
			return "service:my-svc", true
		}),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) { return nil, nil })
	ctx := transportCtx(testOp)
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := fa.lastRequest()
	if req.Subject != "service:my-svc" {
		t.Errorf("Subject = %q, want service:my-svc", req.Subject)
	}
}

// ---------------------------------------------------------------------------
// Tests: WithAuditOnDeny
// ---------------------------------------------------------------------------

func TestServer_AuditOnDeny_DeniedEmitsEvent(t *testing.T) {
	auditor := &captureAuditor{}
	mw := Server(&fakeAuthorizer{allowed: false},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:alice")),
		WithAuditOnDeny(auditor),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on deny")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected denial error")
	}

	events := auditor.getEvents()
	if len(events) != 1 {
		t.Fatalf("emit count = %d, want 1", len(events))
	}
	e := events[0]
	if e.Type() != "servora.authz.v1.denied" {
		t.Errorf("event type = %q, want servora.authz.v1.denied", e.Type())
	}

	var data map[string]any
	if err := e.DataAs(&data); err != nil {
		t.Fatalf("failed to decode event data: %v", err)
	}
	if data["severity"] != "WARN" {
		t.Errorf("severity = %v, want WARN", data["severity"])
	}
	if data["subject"] != "user:alice" {
		t.Errorf("subject = %v, want user:alice", data["subject"])
	}
}

func TestServer_AuditOnDeny_ErrorEmitsEvent(t *testing.T) {
	auditor := &captureAuditor{}
	sentinel := errors.New("backend down")
	mw := Server(&fakeAuthorizer{err: sentinel},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:alice")),
		WithAuditOnDeny(auditor),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on error")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	events := auditor.getEvents()
	if len(events) != 1 {
		t.Fatalf("emit count = %d, want 1", len(events))
	}
	e := events[0]
	if e.Type() != "servora.authz.v1.denied" {
		t.Errorf("event type = %q, want servora.authz.v1.denied", e.Type())
	}

	var data map[string]any
	if err := e.DataAs(&data); err != nil {
		t.Fatalf("failed to decode event data: %v", err)
	}
	if data["severity"] != "ERROR" {
		t.Errorf("severity = %v, want ERROR", data["severity"])
	}
	if data["error"] != sentinel.Error() {
		t.Errorf("error = %v, want %q", data["error"], sentinel.Error())
	}
}

func TestServer_AuditOnDeny_NotConfigured_Silent(t *testing.T) {
	// No WithAuditOnDeny configured — ensure no panic and no emission.
	mw := Server(&fakeAuthorizer{allowed: false},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:alice")),
		// No WithAuditOnDeny — auditor is nil.
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called on deny")
		return nil, nil
	})

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err == nil {
		t.Fatal("expected denial error")
	}
	// If we got here without panic, the nil-auditor path works correctly.
}

func TestServer_AuditOnDeny_AllowedNoEmit(t *testing.T) {
	// When authz allows, audit should NOT emit.
	auditor := &captureAuditor{}
	mw := Server(&fakeAuthorizer{allowed: true},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:alice")),
		WithAuditOnDeny(auditor),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) { return "ok", nil })

	ctx := transportCtx(testOp)
	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := auditor.getEvents()
	if len(events) != 0 {
		t.Errorf("emit count = %d, want 0 (allowed should not emit)", len(events))
	}
}

// ---------------------------------------------------------------------------
// Tests: Check timeout
// ---------------------------------------------------------------------------

// blockingAuthorizer simulates a slow backend by waiting until ctx is cancelled.
type blockingAuthorizer struct{}

func (b *blockingAuthorizer) Check(ctx context.Context, _ CheckRequest) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}

func TestServer_CheckTimeout_TripsCheckBeforeBackend(t *testing.T) {
	mw := Server(
		&blockingAuthorizer{},
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:123")),
		WithCheckTimeout(50*time.Millisecond),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler must not be reached when check times out")
		return nil, nil
	})

	ctx := transportCtx(testOp)
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

// ---------------------------------------------------------------------------
// Tests: Fail-open on missing rule
// ---------------------------------------------------------------------------

func TestServer_FailOpenOnMissingRule_PassesThroughAndAlerts(t *testing.T) {
	var alerted *string
	mw := Server(nil,
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

// ---------------------------------------------------------------------------
// Tests: resolveResource
// ---------------------------------------------------------------------------

func TestResolveResource_ResourceIDField_Empty_UsesDefault(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ResourceType: "platform", ResourceIDField: ""}
	resourceType, resourceID, err := resolveResource(rule, nil, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resourceType != "platform" {
		t.Errorf("resourceType = %q, want platform", resourceType)
	}
	if resourceID != "default" {
		t.Errorf("resourceID = %q, want default", resourceID)
	}
}

func TestResolveResource_ResourceIDField_Empty_CustomDefault(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ResourceType: "platform", ResourceIDField: ""}
	resourceType, resourceID, err := resolveResource(rule, nil, "global")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resourceType != "platform" {
		t.Errorf("resourceType = %q, want platform", resourceType)
	}
	if resourceID != "global" {
		t.Errorf("resourceID = %q, want global", resourceID)
	}
}

func TestResolveResource_ResourceIDField_Set_ExtractedFromProto(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ResourceType: "user", ResourceIDField: "value"}
	req := &wrapperspb.StringValue{Value: "user-abc-123"}

	resourceType, resourceID, err := resolveResource(rule, req, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resourceType != "user" {
		t.Errorf("resourceType = %q, want user", resourceType)
	}
	if resourceID != "user-abc-123" {
		t.Errorf("resourceID = %q, want user-abc-123", resourceID)
	}
}

func TestResolveResource_ResourceIDField_NotFound_Error(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ResourceType: "user", ResourceIDField: "nonexistent_field"}
	req := &wrapperspb.StringValue{Value: "user-abc-123"}

	_, _, err := resolveResource(rule, req, "default")
	if err == nil {
		t.Fatal("expected error for nonexistent field")
	}
}

func TestResolveResource_ResourceType_Empty_Error(t *testing.T) {
	rule := AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, ResourceType: ""}
	_, _, err := resolveResource(rule, nil, "default")
	if err == nil {
		t.Fatal("expected error for empty ResourceType")
	}
}

// ---------------------------------------------------------------------------
// Tests: extractProtoField
// ---------------------------------------------------------------------------

func TestExtractProtoField_NonProtoRequest_Error(t *testing.T) {
	_, err := extractProtoField("not a proto message", "id")
	if err == nil {
		t.Fatal("expected error for non-proto request")
	}
}

func TestExtractProtoField_EmptyFieldValue_Error(t *testing.T) {
	req := &wrapperspb.StringValue{Value: ""} // empty value
	_, err := extractProtoField(req, "value")
	if err == nil {
		t.Fatal("expected error for empty field value")
	}
}

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

func TestExtractProtoField_DotPath_MissingSegment(t *testing.T) {
	req := &auditpb.AuditEvent{
		Target: &auditpb.AuditTarget{Id: "x"},
	}
	_, err := extractProtoField(req, "target.missing")
	if err == nil {
		t.Fatal("expected error for missing nested segment")
	}
}

func TestExtractProtoField_DotPath_TerminatesOnMessage_Errors(t *testing.T) {
	req := &auditpb.AuditEvent{
		Target: &auditpb.AuditTarget{Id: "x"},
	}
	_, err := extractProtoField(req, "target")
	if err == nil {
		t.Fatal("expected error when path terminus is a message, not a scalar")
	}
}

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

// ---------------------------------------------------------------------------
// Tests: WithDefaultResourceID
// ---------------------------------------------------------------------------

func TestServer_WithDefaultResourceID(t *testing.T) {
	fa := &fakeAuthorizer{allowed: true}
	mw := Server(fa,
		WithRules(map[string]AuthzRule{
			testOp: {Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "admin", ResourceType: "platform"},
		}),
		WithSubjectFunc(staticSubjectFunc("user:123")),
		WithDefaultResourceID("global"),
	)

	handler := mw(func(ctx context.Context, req any) (any, error) { return nil, nil })
	ctx := transportCtx(testOp)
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := fa.lastRequest()
	if req.ResourceID != "global" {
		t.Errorf("ResourceID = %q, want global", req.ResourceID)
	}
}

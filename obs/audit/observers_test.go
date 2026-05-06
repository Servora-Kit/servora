package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/Servora-Kit/servora/core/actor"
	"github.com/Servora-Kit/servora/security/authn"
	"github.com/Servora-Kit/servora/security/authz"
)

func TestAuthnObserver_NilRecorder_Noop(t *testing.T) {
	var r *Recorder
	obs := r.AuthnObserver()
	if obs == nil {
		t.Fatal("AuthnObserver() on nil Recorder returned nil func; expected no-op closure")
	}
	// Must not panic.
	obs(context.Background(), authn.AuthnDetail{
		Method:  "jwt",
		Subject: actor.NewAnonymousActor(),
		Allowed: true,
	})
}

func TestAuthnObserver_Success(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")

	user := actor.NewUserActor(actor.UserActorParams{ID: "u-1", DisplayName: "alice"})

	obs := recorder.AuthnObserver()
	obs(newAuditTestContext("/audit.v1.Test/Login"), authn.AuthnDetail{
		Method:  "jwt",
		Subject: user,
		Allowed: true,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(emitter.events))
	}
	evt := emitter.events[0]
	if evt.EventType != EventTypeAuthnResult {
		t.Fatalf("event type = %q, want %q", evt.EventType, EventTypeAuthnResult)
	}
	detail, ok := evt.Detail.(AuthnDetail)
	if !ok {
		t.Fatalf("expected AuthnDetail, got %T", evt.Detail)
	}
	if detail.Method != "jwt" {
		t.Errorf("detail.Method = %q, want %q", detail.Method, "jwt")
	}
	if !detail.Success {
		t.Errorf("detail.Success = false, want true")
	}
	if detail.FailureReason != "" {
		t.Errorf("detail.FailureReason = %q, want empty", detail.FailureReason)
	}
	if !evt.Result.Success {
		t.Errorf("evt.Result.Success = false, want true")
	}
	if evt.Operation != "/audit.v1.Test/Login" {
		t.Errorf("operation = %q, want resolved from transport", evt.Operation)
	}
	if evt.Actor.ID != "u-1" {
		t.Errorf("actor.ID = %q, want %q", evt.Actor.ID, "u-1")
	}
}

func TestAuthnObserver_Failure(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")

	bad := errors.New("invalid token")

	obs := recorder.AuthnObserver()
	obs(context.Background(), authn.AuthnDetail{
		Method:  "jwt",
		Subject: actor.NewAnonymousActor(),
		Allowed: false,
		Err:     bad,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(emitter.events))
	}
	evt := emitter.events[0]
	detail, ok := evt.Detail.(AuthnDetail)
	if !ok {
		t.Fatalf("expected AuthnDetail, got %T", evt.Detail)
	}
	if detail.Success {
		t.Errorf("detail.Success = true, want false")
	}
	if detail.FailureReason != bad.Error() {
		t.Errorf("detail.FailureReason = %q, want %q", detail.FailureReason, bad.Error())
	}
	if evt.Result.Success {
		t.Errorf("evt.Result.Success = true, want false")
	}
	if evt.Result.ErrorMessage != bad.Error() {
		t.Errorf("evt.Result.ErrorMessage = %q, want %q", evt.Result.ErrorMessage, bad.Error())
	}
}

func TestAuthzObserver_NilRecorder_Noop(t *testing.T) {
	var r *Recorder
	obs := r.AuthzObserver()
	if obs == nil {
		t.Fatal("AuthzObserver() on nil Recorder returned nil func; expected no-op closure")
	}
	// Must not panic.
	obs(context.Background(), authz.DecisionDetail{
		Operation:  "/audit.v1.Test/Get",
		Subject:    "user:u1",
		Relation:   "viewer",
		ObjectType: "project",
		ObjectID:   "p1",
		Allowed:    true,
	})
}

func TestAuthzObserver_DecisionMapping(t *testing.T) {
	someErr := errors.New("openfga timeout")

	tests := []struct {
		name           string
		detail         authz.DecisionDetail
		wantDecision   AuthzDecision
		wantSuccess    bool
		wantErrMessage string
	}{
		{
			name: "allowed",
			detail: authz.DecisionDetail{
				Operation:  "/audit.v1.Test/Get",
				Subject:    "user:u1",
				Relation:   "viewer",
				ObjectType: "project",
				ObjectID:   "p1",
				Allowed:    true,
			},
			wantDecision: AuthzDecisionAllowed,
			wantSuccess:  true,
		},
		{
			name: "denied",
			detail: authz.DecisionDetail{
				Operation:  "/audit.v1.Test/Get",
				Subject:    "user:u1",
				Relation:   "admin",
				ObjectType: "project",
				ObjectID:   "p1",
				Allowed:    false,
			},
			wantDecision: AuthzDecisionDenied,
			wantSuccess:  false,
		},
		{
			name: "error_wins_over_allowed",
			detail: authz.DecisionDetail{
				Operation:  "/audit.v1.Test/Get",
				Subject:    "user:u1",
				Relation:   "viewer",
				ObjectType: "project",
				ObjectID:   "p1",
				Allowed:    true,
				Err:        someErr,
			},
			wantDecision:   AuthzDecisionError,
			wantSuccess:    false,
			wantErrMessage: someErr.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			recorder := NewRecorder(emitter, "iam")
			obs := recorder.AuthzObserver()

			ctx := actor.NewContext(context.Background(), actor.NewUserActor(actor.UserActorParams{ID: "u1"}))
			obs(ctx, tt.detail)

			if len(emitter.events) != 1 {
				t.Fatalf("expected 1 audit event, got %d", len(emitter.events))
			}
			evt := emitter.events[0]
			if evt.EventType != EventTypeAuthzDecision {
				t.Fatalf("event type = %q, want %q", evt.EventType, EventTypeAuthzDecision)
			}
			detail, ok := evt.Detail.(AuthzDetail)
			if !ok {
				t.Fatalf("expected AuthzDetail, got %T", evt.Detail)
			}
			if detail.Decision != tt.wantDecision {
				t.Errorf("detail.Decision = %q, want %q", detail.Decision, tt.wantDecision)
			}
			if detail.Relation != tt.detail.Relation {
				t.Errorf("detail.Relation = %q, want %q", detail.Relation, tt.detail.Relation)
			}
			if detail.ObjectType != tt.detail.ObjectType {
				t.Errorf("detail.ObjectType = %q, want %q", detail.ObjectType, tt.detail.ObjectType)
			}
			if detail.ObjectID != tt.detail.ObjectID {
				t.Errorf("detail.ObjectID = %q, want %q", detail.ObjectID, tt.detail.ObjectID)
			}
			if evt.Result.Success != tt.wantSuccess {
				t.Errorf("evt.Result.Success = %v, want %v", evt.Result.Success, tt.wantSuccess)
			}
			if evt.Result.ErrorMessage != tt.wantErrMessage {
				t.Errorf("evt.Result.ErrorMessage = %q, want %q", evt.Result.ErrorMessage, tt.wantErrMessage)
			}
			if evt.Operation != tt.detail.Operation {
				t.Errorf("operation = %q, want %q", evt.Operation, tt.detail.Operation)
			}
		})
	}
}

func TestAuthzObserver_AnonymousActor_Fallback(t *testing.T) {
	emitter := &captureEmitter{}
	recorder := NewRecorder(emitter, "iam")
	obs := recorder.AuthzObserver()

	// Empty context: no actor injected — observer should fall back to anonymous.
	obs(context.Background(), authz.DecisionDetail{
		Operation:  "/audit.v1.Test/Get",
		Subject:    "user:unknown",
		Relation:   "viewer",
		ObjectType: "project",
		ObjectID:   "p1",
		Allowed:    false,
	})

	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(emitter.events))
	}
	evt := emitter.events[0]
	if evt.Actor.Type != string(actor.TypeAnonymous) {
		t.Errorf("actor.Type = %q, want %q", evt.Actor.Type, actor.TypeAnonymous)
	}
}

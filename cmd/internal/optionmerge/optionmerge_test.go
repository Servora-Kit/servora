package optionmerge

import (
	"testing"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	authnpb "github.com/Servora-Kit/servora/api/gen/go/servora/authn/v1"
	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
)

func TestMerge_Authn(t *testing.T) {
	tests := []struct {
		name       string
		svc        *authnpb.AuthnRule
		method     *authnpb.AuthnRule
		hasMethod  bool
		wantOK     bool
		wantMode   authnpb.AuthnRule_Mode
		wantScheme string // first scheme, if any
	}{
		{
			name:      "both nil returns false",
			svc:       nil,
			method:    nil,
			hasMethod: false,
			wantOK:    false,
		},
		{
			name:       "service default wins when no method rule",
			svc:        &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_REQUIRED, Schemes: []string{"bearer"}},
			method:     nil,
			hasMethod:  false,
			wantOK:     true,
			wantMode:   authnpb.AuthnRule_MODE_REQUIRED,
			wantScheme: "bearer",
		},
		{
			name:       "method rule wins when mode is non-zero",
			svc:        &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_REQUIRED, Schemes: []string{"bearer"}},
			method:     &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_PUBLIC},
			hasMethod:  true,
			wantOK:     true,
			wantMode:   authnpb.AuthnRule_MODE_PUBLIC,
			wantScheme: "",
		},
		{
			name:       "method with UNSPECIFIED mode inherits service default",
			svc:        &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_REQUIRED, Schemes: []string{"mtls"}},
			method:     &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_UNSPECIFIED},
			hasMethod:  true,
			wantOK:     true,
			wantMode:   authnpb.AuthnRule_MODE_REQUIRED,
			wantScheme: "mtls",
		},
		{
			name:      "service default with UNSPECIFIED mode returns false",
			svc:       &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_UNSPECIFIED},
			method:    nil,
			hasMethod: false,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Merge(tt.svc, tt.method, tt.hasMethod)
			if ok != tt.wantOK {
				t.Fatalf("Merge() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Mode != tt.wantMode {
				t.Errorf("Merge() mode = %v, want %v", got.Mode, tt.wantMode)
			}
			firstScheme := ""
			if len(got.Schemes) > 0 {
				firstScheme = got.Schemes[0]
			}
			if firstScheme != tt.wantScheme {
				t.Errorf("Merge() scheme[0] = %q, want %q", firstScheme, tt.wantScheme)
			}
		})
	}
}

func TestMerge_Authz(t *testing.T) {
	tests := []struct {
		name      string
		svc       *authzpb.AuthzRule
		method    *authzpb.AuthzRule
		hasMethod bool
		wantOK    bool
		wantMode  authzpb.AuthzMode
	}{
		{
			name:      "both nil returns false",
			svc:       nil,
			method:    nil,
			hasMethod: false,
			wantOK:    false,
		},
		{
			name:      "service default wins",
			svc:       &authzpb.AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK, Action: "read"},
			method:    nil,
			hasMethod: false,
			wantOK:    true,
			wantMode:  authzpb.AuthzMode_AUTHZ_MODE_CHECK,
		},
		{
			name:      "method rule wins",
			svc:       &authzpb.AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK},
			method:    &authzpb.AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_NONE},
			hasMethod: true,
			wantOK:    true,
			wantMode:  authzpb.AuthzMode_AUTHZ_MODE_NONE,
		},
		{
			name:      "method UNSPECIFIED inherits service",
			svc:       &authzpb.AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_CHECK},
			method:    &authzpb.AuthzRule{Mode: authzpb.AuthzMode_AUTHZ_MODE_UNSPECIFIED},
			hasMethod: true,
			wantOK:    true,
			wantMode:  authzpb.AuthzMode_AUTHZ_MODE_CHECK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Merge(tt.svc, tt.method, tt.hasMethod)
			if ok != tt.wantOK {
				t.Fatalf("Merge() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Mode != tt.wantMode {
				t.Errorf("Merge() mode = %v, want %v", got.Mode, tt.wantMode)
			}
		})
	}
}

func TestMerge_Audit(t *testing.T) {
	tests := []struct {
		name      string
		svc       *auditv1.AuditRule
		method    *auditv1.AuditRule
		hasMethod bool
		wantOK    bool
		wantMode  auditv1.AuditMode
	}{
		{
			name:      "both nil returns false",
			svc:       nil,
			method:    nil,
			hasMethod: false,
			wantOK:    false,
		},
		{
			name:      "service default wins",
			svc:       &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED, EventType: "admin.create"},
			method:    nil,
			hasMethod: false,
			wantOK:    true,
			wantMode:  auditv1.AuditMode_AUDIT_MODE_ENABLED,
		},
		{
			name:      "method rule wins",
			svc:       &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED},
			method:    &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_DISABLED},
			hasMethod: true,
			wantOK:    true,
			wantMode:  auditv1.AuditMode_AUDIT_MODE_DISABLED,
		},
		{
			name:      "method UNSPECIFIED inherits service",
			svc:       &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED},
			method:    &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_UNSPECIFIED},
			hasMethod: true,
			wantOK:    true,
			wantMode:  auditv1.AuditMode_AUDIT_MODE_ENABLED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Merge(tt.svc, tt.method, tt.hasMethod)
			if ok != tt.wantOK {
				t.Fatalf("Merge() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Mode != tt.wantMode {
				t.Errorf("Merge() mode = %v, want %v", got.Mode, tt.wantMode)
			}
		})
	}
}

func TestMerge_DeepClone(t *testing.T) {
	original := &authnpb.AuthnRule{
		Mode:    authnpb.AuthnRule_MODE_REQUIRED,
		Schemes: []string{"bearer", "mtls"},
	}
	merged, ok := Merge[*authnpb.AuthnRule](nil, original, true)
	if !ok {
		t.Fatal("expected ok")
	}

	// Mutate the returned value — original must be unaffected.
	merged.Schemes[0] = "MUTATED"
	if original.Schemes[0] == "MUTATED" {
		t.Fatal("Merge did not deep-clone; original was mutated")
	}
}

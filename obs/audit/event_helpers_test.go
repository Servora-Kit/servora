package audit

import (
	"context"
	"testing"

	kratos "github.com/go-kratos/kratos/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockAppInfo implements kratos.AppInfo for testing.
type mockAppInfo struct {
	name string
}

func (m mockAppInfo) ID() string                  { return "test-id" }
func (m mockAppInfo) Name() string                { return m.name }
func (m mockAppInfo) Version() string             { return "v0.0.0" }
func (m mockAppInfo) Metadata() map[string]string { return nil }
func (m mockAppInfo) Endpoint() []string          { return nil }

func TestNewEvent_Defaults(t *testing.T) {
	ctx := context.Background()
	e := NewEvent(ctx)

	if e.SpecVersion() != "1.0" {
		t.Errorf("SpecVersion = %q, want %q", e.SpecVersion(), "1.0")
	}
	if e.ID() == "" {
		t.Error("ID should not be empty")
	}
	if e.Time().IsZero() {
		t.Error("Time should not be zero")
	}

	// No app context: source should fall back to "//unknown".
	if e.Source() != "//unknown" {
		t.Errorf("Source = %q, want %q", e.Source(), "//unknown")
	}

	// severity and recordedtime should NOT be set by default.
	if _, ok := e.Extensions()["severitytext"]; ok {
		t.Error("severitytext extension should not be set by NewEvent")
	}
	if _, ok := e.Extensions()["recordedtime"]; ok {
		t.Error("recordedtime extension should not be set by NewEvent")
	}
}

func TestNewEvent_SourceFromAppContext(t *testing.T) {
	ctx := kratos.NewContext(context.Background(), mockAppInfo{name: "myapp"})
	e := NewEvent(ctx)

	if e.Source() != "//myapp" {
		t.Errorf("Source = %q, want %q", e.Source(), "//myapp")
	}
}

func TestNewEvent_SourceFallback(t *testing.T) {
	ctx := context.Background()
	e := NewEvent(ctx)

	if e.Source() != "//unknown" {
		t.Errorf("Source = %q, want %q", e.Source(), "//unknown")
	}
}

func TestNewEvent_WithOptions(t *testing.T) {
	ctx := context.Background()
	e := NewEvent(ctx,
		WithType("test.type"),
		WithSource("test-source"),
		WithSubject("test-subject"),
	)

	if e.Type() != "test.type" {
		t.Errorf("Type = %q, want %q", e.Type(), "test.type")
	}
	if e.Source() != "test-source" {
		t.Errorf("Source = %q, want %q", e.Source(), "test-source")
	}
	if e.Subject() != "test-subject" {
		t.Errorf("Subject = %q, want %q", e.Subject(), "test-subject")
	}
}

func TestSetProtoData(t *testing.T) {
	ctx := context.Background()
	e := NewEvent(ctx, WithType("test.proto"))

	// Use a well-known proto message for testing.
	msg := timestamppb.Now()

	if err := SetProtoData(&e, msg); err != nil {
		t.Fatalf("SetProtoData failed: %v", err)
	}

	if e.DataContentType() != "application/protobuf" {
		t.Errorf("DataContentType = %q, want %q", e.DataContentType(), "application/protobuf")
	}

	wantSchema := "type.googleapis.com/google.protobuf.Timestamp"
	if e.DataSchema() != wantSchema {
		t.Errorf("DataSchema = %q, want %q", e.DataSchema(), wantSchema)
	}

	if len(e.Data()) == 0 {
		t.Error("Data should not be empty after SetProtoData")
	}
}

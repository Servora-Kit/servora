package audit

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"
)

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

	sev, ok := e.Extensions()[ExtSeverityText]
	if !ok {
		t.Error("severity text extension should be set")
	}
	if sev != "INFO" {
		t.Errorf("severity = %v, want INFO", sev)
	}

	rec, ok := e.Extensions()[ExtRecordedTime]
	if !ok {
		t.Error("recorded time extension should be set")
	}
	if rec == "" {
		t.Error("recorded time should not be empty")
	}
}

func TestNewEvent_WithOptions(t *testing.T) {
	ctx := context.Background()
	e := NewEvent(ctx,
		WithType("test.type"),
		WithSource("test-source"),
		WithSeverity("ERROR"),
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
	sev := e.Extensions()[ExtSeverityText]
	if sev != "ERROR" {
		t.Errorf("severity = %v, want ERROR", sev)
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

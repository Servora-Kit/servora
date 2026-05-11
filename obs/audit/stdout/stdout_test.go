package stdout

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/Servora-Kit/servora/obs/audit"
)

func TestNewAuditor_ImplementsInterface(t *testing.T) {
	var _ audit.Auditor = NewAuditor()
}

func TestAuditor_EmitWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	a := NewAuditor(WithWriter(&buf))

	e := cloudevents.NewEvent()
	e.SetType("test.type")
	e.SetSource("test-source")
	e.SetID("test-id")

	err := a.Emit(context.Background(), e)
	if err != nil {
		t.Fatalf("Emit() returned error: %v", err)
	}

	// Verify output is valid JSON.
	output := buf.Bytes()
	if len(output) == 0 {
		t.Fatal("expected output, got empty")
	}

	var parsed map[string]interface{}
	// Remove trailing newline for JSON parse.
	if err := json.Unmarshal(bytes.TrimRight(output, "\n"), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if parsed["type"] != "test.type" {
		t.Errorf("type = %v, want test.type", parsed["type"])
	}
	if parsed["source"] != "test-source" {
		t.Errorf("source = %v, want test-source", parsed["source"])
	}
}

func TestAuditor_EmitEndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	a := NewAuditor(WithWriter(&buf))

	e := cloudevents.NewEvent()
	e.SetType("test")
	e.SetSource("test")

	if err := a.Emit(context.Background(), e); err != nil {
		t.Fatalf("Emit() failed: %v", err)
	}

	output := buf.Bytes()
	if len(output) == 0 || output[len(output)-1] != '\n' {
		t.Error("output should end with newline")
	}
}

package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/Servora-Kit/servora/obs/audit"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var _ audit.Auditor = (*auditorImpl)(nil)

func TestAuditor_EmitMapsCloudEventFields(t *testing.T) {
	var buf bytes.Buffer
	a := NewAuditor(slog.New(slog.NewJSONHandler(&buf, nil)))

	eventTime := time.Date(2026, 5, 23, 10, 11, 12, 13000000, time.UTC)
	e := cloudevents.NewEvent()
	e.SetID("test-id")
	e.SetType("test.type")
	e.SetSource("test-source")
	e.SetSubject("test-subject")
	e.SetTime(eventTime)
	// Add an arbitrary extension to verify generic extension output.
	e.SetExtension("customkey", "customval")

	if err := a.Emit(context.Background(), e); err != nil {
		t.Fatalf("Emit() returned error: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if record["msg"] != "audit_event" {
		t.Fatalf("msg = %v, want audit_event", record["msg"])
	}

	cloudEvent, ok := record["cloudevent"].(map[string]any)
	if !ok {
		t.Fatalf("cloudevent = %T, want object", record["cloudevent"])
	}

	// Verify CE required attributes + subject.
	wantFixed := map[string]any{
		"id":      "test-id",
		"type":    "test.type",
		"source":  "test-source",
		"subject": "test-subject",
		"time":    eventTime.Format(time.RFC3339Nano),
	}
	for key, value := range wantFixed {
		if cloudEvent[key] != value {
			t.Errorf("cloudevent.%s = %v, want %v", key, cloudEvent[key], value)
		}
	}

	// Verify extension is present via generic iteration.
	if cloudEvent["customkey"] != "customval" {
		t.Errorf("cloudevent.customkey = %v, want customval", cloudEvent["customkey"])
	}

	// Verify that old hardcoded extension names are NOT required in the output;
	// the log backend no longer forces severitytext or recordedtime.
}

func TestAuditor_EmitWithMultipleExtensions(t *testing.T) {
	var buf bytes.Buffer
	a := NewAuditor(slog.New(slog.NewJSONHandler(&buf, nil)))

	e := cloudevents.NewEvent()
	e.SetID("ext-id")
	e.SetType("ext.type")
	e.SetSource("//myapp")
	e.SetSubject("/my.Service/Method")
	e.SetTime(time.Now())
	e.SetExtension("traceparent", "00-abc-def-01")
	e.SetExtension("errormessage", "some error")

	if err := a.Emit(context.Background(), e); err != nil {
		t.Fatalf("Emit() returned error: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	cloudEvent := record["cloudevent"].(map[string]any)

	if cloudEvent["traceparent"] != "00-abc-def-01" {
		t.Errorf("traceparent = %v, want 00-abc-def-01", cloudEvent["traceparent"])
	}
	if cloudEvent["errormessage"] != "some error" {
		t.Errorf("errormessage = %v, want some error", cloudEvent["errormessage"])
	}
}

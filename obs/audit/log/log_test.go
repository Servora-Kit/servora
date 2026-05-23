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

func TestNewAuditor_ImplementsInterface(t *testing.T) {
	var _ audit.Auditor = NewAuditor(slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)))
}

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
	e.SetExtension(audit.ExtSeverityText, "WARN")
	e.SetExtension(audit.ExtRecordedTime, "2026-05-23T10:11:13.014Z")

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

	want := map[string]any{
		"id":                  "test-id",
		"type":                "test.type",
		"source":              "test-source",
		"subject":             "test-subject",
		"time":                eventTime.Format(time.RFC3339Nano),
		audit.ExtSeverityText: "WARN",
		audit.ExtRecordedTime: "2026-05-23T10:11:13.014Z",
	}
	for key, value := range want {
		if cloudEvent[key] != value {
			t.Errorf("cloudevent.%s = %v, want %v", key, cloudEvent[key], value)
		}
	}
}

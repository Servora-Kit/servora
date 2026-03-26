package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteEvent(t *testing.T) {
	rec := httptest.NewRecorder()
	err := WriteEvent(rec, Event{ID: "1", Event: "message", Data: "hello"})
	if err != nil {
		t.Fatalf("WriteEvent error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "id: 1\n") {
		t.Fatalf("unexpected body: %q", body)
	}
	if !strings.Contains(body, "event: message\n") {
		t.Fatalf("unexpected body: %q", body)
	}
	if !strings.Contains(body, "data: hello\n\n") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestNewStaticHandler(t *testing.T) {
	h := NewStaticHandler(Event{Event: "ready", Data: "servora"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/events", nil)
	h(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "event: ready") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestNewTickerHandler(t *testing.T) {
	h := NewTickerHandler(1*time.Millisecond, 2)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/ticks", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	h(rec, req)
	body := rec.Body.String()

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q", got)
	}
	if strings.Count(body, "event: tick") < 2 {
		t.Fatalf("expected at least 2 tick events, body=%q", body)
	}
}

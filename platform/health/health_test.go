package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string                  { return m.name }
func (m *mockChecker) Check(_ context.Context) error { return m.err }

func parseJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	return result
}

func TestLivenessHandler_AlwaysReturns200(t *testing.T) {
	h := NewBuilder().Build()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.LivenessHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	result := parseJSON(t, w.Body.Bytes())
	if result["status"] != "alive" {
		t.Fatalf("expected status=alive, got %v", result["status"])
	}
}

func TestLivenessHandler_IgnoresCheckers(t *testing.T) {
	h := NewBuilder().WithChecker(&mockChecker{name: "failing", err: errors.New("down")}).Build()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.LivenessHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("liveness should ignore checkers, got %d", w.Code)
	}
}

func TestReadinessHandler_NoCheckers(t *testing.T) {
	h := NewBuilder().Build()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	h.ReadinessHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with no checkers, got %d", len(h.checkers))
	}
}

func TestNewHandlerWithDefaults_NilDeps(t *testing.T) {
	h := NewHandlerWithDefaults(DefaultDeps{})
	if len(h.checkers) != 0 {
		t.Fatalf("expected 0 checkers, got %d", len(h.checkers))
	}
}

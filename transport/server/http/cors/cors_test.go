package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/extra/cors/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TestApplyDefaults_PluginGenerated verifies the proto-sourced defaults
// supplied by protoc-gen-servora-conf on extra/cors/v1/cors.proto, which
// replaces the prior hand-coded defaultOptions().
func TestApplyDefaults_PluginGenerated(t *testing.T) {
	c := &corsv1.CORS{}
	c.ApplyDefaults()

	if len(c.AllowedOrigins) != 1 || c.AllowedOrigins[0] != "*" {
		t.Errorf("default allowed origins = %v, want [\"*\"]", c.AllowedOrigins)
	}
	want := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "OPTIONS": true}
	if len(c.AllowedMethods) != len(want) {
		t.Errorf("default allowed methods count = %d, want %d", len(c.AllowedMethods), len(want))
	}
	for _, m := range c.AllowedMethods {
		if !want[m] {
			t.Errorf("unexpected default method %q", m)
		}
	}
	if c.MaxAge == nil || c.MaxAge.AsDuration() != 24*time.Hour {
		t.Errorf("default max age = %v, want 24h", c.MaxAge)
	}
}

func TestMiddleware_NilConfig(t *testing.T) {
	corsMiddleware := Middleware(nil)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if w.Result().Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("nil config should not emit CORS headers")
	}
}

func TestMiddleware_Disabled(t *testing.T) {
	corsConfig := &corsv1.CORS{Enable: false}
	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if w.Result().Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("disabled config should not emit CORS headers")
	}
}

func TestMiddleware_EnabledWithDefaults(t *testing.T) {
	corsConfig := &corsv1.CORS{Enable: true}
	corsConfig.ApplyDefaults() // caller is expected to apply defaults before passing in
	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if got := w.Result().Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
}

func TestMiddleware_SimpleRequest(t *testing.T) {
	corsConfig := &corsv1.CORS{
		Enable:           true,
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           durationpb.New(time.Hour),
	}

	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	res := w.Result()
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := res.Header.Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST")
	}
	if res.Header.Get("Access-Control-Allow-Credentials") == "true" {
		t.Error("expected no Access-Control-Allow-Credentials header")
	}
}

func TestMiddleware_PreflightRequest(t *testing.T) {
	corsConfig := &corsv1.CORS{
		Enable:           true,
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           durationpb.New(time.Hour),
	}

	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("OPTIONS", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("preflight status = %d, want %d", res.StatusCode, http.StatusNoContent)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
	if got := res.Header.Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, "3600")
	}
}

func TestMiddleware_OriginNotAllowed(t *testing.T) {
	corsConfig := &corsv1.CORS{
		Enable:         true,
		AllowedOrigins: []string{"https://allowed.com"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}

	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://notallowed.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if w.Result().Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no Access-Control-Allow-Origin header for disallowed origin")
	}
}

func TestMiddleware_WithCredentials(t *testing.T) {
	corsConfig := &corsv1.CORS{
		Enable:           true,
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}

	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	if got := w.Result().Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
}

func TestIsEnabled(t *testing.T) {
	withDefaults := &corsv1.CORS{Enable: true}
	withDefaults.ApplyDefaults()
	tests := []struct {
		name     string
		config   *corsv1.CORS
		expected bool
	}{
		{"nil config", nil, false},
		{"disabled", &corsv1.CORS{Enable: false}, false},
		{"enabled but no origins", &corsv1.CORS{Enable: true}, false}, // defaults not applied
		{"enabled with defaults", withDefaults, true},
		{"enabled with origins", &corsv1.CORS{Enable: true, AllowedOrigins: []string{"https://example.com"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEnabled(tt.config); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetAllowedOrigins(t *testing.T) {
	withDefaults := &corsv1.CORS{Enable: true}
	withDefaults.ApplyDefaults()
	tests := []struct {
		name     string
		config   *corsv1.CORS
		expected []string
	}{
		{"nil config", nil, nil},
		{"disabled returns empty list", &corsv1.CORS{Enable: false}, nil},
		{"enabled with defaults applied", withDefaults, []string{"*"}},
		{"custom origins", &corsv1.CORS{Enable: true, AllowedOrigins: []string{"https://a.com", "https://b.com"}}, []string{"https://a.com", "https://b.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAllowedOrigins(tt.config)
			if len(got) != len(tt.expected) {
				t.Errorf("GetAllowedOrigins() = %v, want %v", got, tt.expected)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("GetAllowedOrigins() = %v, want %v", got, tt.expected)
					return
				}
			}
		})
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name          string
		origin        string
		allowedOrigin []string
		expected      bool
	}{
		{"wildcard", "https://example.com", []string{"*"}, true},
		{"exact match", "https://example.com", []string{"https://example.com"}, true},
		{"no match", "https://example.com", []string{"https://different.com"}, false},
		{"empty origin", "", []string{"*"}, false},
		{"wildcard subdomain", "https://api.example.com", []string{"*.example.com"}, true},
		{"wildcard subdomain no match", "https://api.baddomain.com", []string{"*.example.com"}, false},
		{"wildcard subdomain too many dots", "https://fake.api.example.com", []string{"*.example.com"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOriginAllowed(tt.origin, tt.allowedOrigin); got != tt.expected {
				t.Errorf("isOriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowedOrigin, got, tt.expected)
			}
		})
	}
}

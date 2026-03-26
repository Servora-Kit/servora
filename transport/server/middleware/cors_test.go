package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	if len(opts.AllowedOrigins) != 1 || opts.AllowedOrigins[0] != "*" {
		t.Errorf("Expected default allowed origins to be ['*'], got %v", opts.AllowedOrigins)
	}

	if len(opts.AllowedMethods) != 5 {
		t.Errorf("Expected 5 default allowed methods, got %d", len(opts.AllowedMethods))
	}

	if !contains(opts.AllowedMethods, "GET") || !contains(opts.AllowedMethods, "POST") {
		t.Errorf("Expected GET and POST in default allowed methods, got %v", opts.AllowedMethods)
	}

	if opts.AllowCredentials {
		t.Error("Expected default allow credentials to be false")
	}

	if opts.MaxAge != 24*time.Hour {
		t.Errorf("Expected default max age to be 24h, got %v", opts.MaxAge)
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

	res := w.Result()
	if res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers when config is nil")
	}
}

func TestMiddleware_Disabled(t *testing.T) {
	corsConfig := &conf.CORS{Enable: false}
	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS headers when disabled")
	}
}

func TestMiddleware_EnabledWithDefaults(t *testing.T) {
	corsConfig := &conf.CORS{Enable: true}
	corsMiddleware := Middleware(corsConfig)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(w, req)

	res := w.Result()
	if res.Header.Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected 'https://example.com' in Access-Control-Allow-Origin, got %s", res.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddleware_SimpleRequest(t *testing.T) {
	corsConfig := &conf.CORS{
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
	if res.Header.Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected 'https://example.com' in Access-Control-Allow-Origin, got %s", res.Header.Get("Access-Control-Allow-Origin"))
	}

	if res.Header.Get("Access-Control-Allow-Methods") != "GET, POST" {
		t.Errorf("Expected 'GET, POST' in Access-Control-Allow-Methods, got %s", res.Header.Get("Access-Control-Allow-Methods"))
	}

	if res.Header.Get("Access-Control-Allow-Credentials") == "true" {
		t.Error("Expected no Access-Control-Allow-Credentials header")
	}
}

func TestMiddleware_PreflightRequest(t *testing.T) {
	corsConfig := &conf.CORS{
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
		t.Errorf("Expected status 204 for preflight request, got %d", res.StatusCode)
	}

	if res.Header.Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Expected 'https://example.com' in Access-Control-Allow-Origin, got %s", res.Header.Get("Access-Control-Allow-Origin"))
	}

	if res.Header.Get("Access-Control-Max-Age") != "3600" {
		t.Errorf("Expected '3600' in Access-Control-Max-Age, got %s", res.Header.Get("Access-Control-Max-Age"))
	}
}

func TestMiddleware_OriginNotAllowed(t *testing.T) {
	corsConfig := &conf.CORS{
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

	res := w.Result()
	if res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no Access-Control-Allow-Origin header when origin is not allowed")
	}
}

func TestMiddleware_WithCredentials(t *testing.T) {
	corsConfig := &conf.CORS{
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

	res := w.Result()
	if res.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Expected Access-Control-Allow-Credentials to be 'true'")
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.CORS
		expected bool
	}{
		{"nil config", nil, false},
		{"disabled", &conf.CORS{Enable: false}, false},
		{"enabled with defaults", &conf.CORS{Enable: true}, true},
		{"enabled with origins", &conf.CORS{Enable: true, AllowedOrigins: []string{"https://example.com"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEnabled(tt.config)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetAllowedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		config   *conf.CORS
		expected []string
	}{
		{"nil config", nil, nil},
		{"disabled", &conf.CORS{Enable: false}, nil},
		{"enabled with defaults", &conf.CORS{Enable: true}, []string{"*"}},
		{"custom origins", &conf.CORS{Enable: true, AllowedOrigins: []string{"https://a.com", "https://b.com"}}, []string{"https://a.com", "https://b.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAllowedOrigins(tt.config)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("Expected %v, got %v", tt.expected, result)
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
			result := isOriginAllowed(tt.origin, tt.allowedOrigin)
			if result != tt.expected {
				t.Errorf("Expected %v for origin %s in %v, got %v", tt.expected, tt.origin, tt.allowedOrigin, result)
			}
		})
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

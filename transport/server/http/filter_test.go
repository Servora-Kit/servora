package http

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestFiltersCoverHandlersInOrder(t *testing.T) {
	var calls []string
	filter := func(name string) khttp.FilterFunc {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, name+" before")
				next.ServeHTTP(w, r)
				calls = append(calls, name+" after")
			})
		}
	}
	srv := NewServer(WithFilter(filter("first")), WithFilter(filter("second")), WithServices(func(s *khttp.Server) {
		s.HandleFunc("/direct", func(w http.ResponseWriter, _ *http.Request) {
			calls = append(calls, "handler")
			w.WriteHeader(http.StatusNoContent)
		})
		// 生成的 HTTP registrar 使用同一个 Route/Context 接口。
		s.Route("/").GET("/generated", func(ctx khttp.Context) error {
			calls = append(calls, "handler")
			ctx.Response().WriteHeader(http.StatusNoContent)
			return nil
		})
	}))
	for _, path := range []string{"/direct", "/generated"} {
		calls = nil
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		want := []string{"first before", "second before", "handler", "second after", "first after"}
		if w.Code != http.StatusNoContent || !reflect.DeepEqual(calls, want) {
			t.Fatalf("%s: status=%d calls=%v", path, w.Code, calls)
		}
	}
}

func TestCORSPreflightPrecedesFilters(t *testing.T) {
	called := false
	srv := NewServer(WithCORS(&corsv1.CORS{Enable: true, AllowedOrigins: []string{"https://example.test"}, AllowedMethods: []string{"GET"}}), WithFilter(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		})
	}))
	request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	request.Header.Set("Origin", "https://example.test")
	request.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, request)
	if called || w.Code >= 400 || w.Header().Get("Access-Control-Allow-Origin") != "https://example.test" {
		t.Fatalf("preflight: called=%v response=%v", called, w.Result())
	}
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if !called || w.Code != http.StatusServiceUnavailable {
		t.Fatalf("filter error: called=%v status=%d", called, w.Code)
	}
}

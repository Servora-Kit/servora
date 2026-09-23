package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	apidocsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/apidocs/v1"
	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
	bootstrapconfig "github.com/Servora-Kit/servora/core/bootstrap/config"
	"github.com/Servora-Kit/servora/transport/server/http/health"
)

func TestJSONCodecUsesProtoJSONForMessages(t *testing.T) {
	NewServer()
	codec := encoding.GetCodec("json")
	if codec == nil {
		t.Fatal("expected json codec to be registered")
	}

	const maxInt64 = int64(9223372036854775807)
	encoded, err := codec.Marshal(wrapperspb.Int64(maxInt64))
	if err != nil {
		t.Fatalf("marshal int64 wrapper: %v", err)
	}
	if got, want := string(encoded), `"9223372036854775807"`; got != want {
		t.Fatalf("expected ProtoJSON int64 string %s, got %s", want, got)
	}

	var decoded wrapperspb.Int64Value
	if err := codec.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal int64 wrapper: %v", err)
	}
	if decoded.Value != maxInt64 {
		t.Fatalf("expected %d, got %d", maxInt64, decoded.Value)
	}
}

func TestJSONCodecPreservesStandardJSONForNonMessages(t *testing.T) {
	NewServer()
	type payload struct {
		DisplayName string `json:"display_name"`
		Count       int    `json:"count"`
	}

	codec := encoding.GetCodec("json")
	if codec == nil {
		t.Fatal("expected json codec to be registered")
	}

	encoded, err := codec.Marshal(payload{DisplayName: "Ada", Count: 7})
	if err != nil {
		t.Fatalf("marshal ordinary JSON payload: %v", err)
	}
	if got, want := string(encoded), `{"display_name":"Ada","count":7}`; got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}

	var decoded payload
	if err := codec.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal ordinary JSON payload: %v", err)
	}
	if decoded.DisplayName != "Ada" || decoded.Count != 7 {
		t.Fatalf("unexpected decoded payload: %+v", decoded)
	}
}

func TestProtoJSONCodecRemainsUsable(t *testing.T) {
	codec := encoding.GetCodec("protojson")
	if codec == nil {
		t.Fatal("expected protojson codec to be registered")
	}

	const maxInt64 = int64(9223372036854775807)
	encoded, err := codec.Marshal(wrapperspb.Int64(maxInt64))
	if err != nil {
		t.Fatalf("marshal int64 wrapper: %v", err)
	}

	var decoded wrapperspb.Int64Value
	if err := codec.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal int64 wrapper: %v", err)
	}
	if decoded.Value != maxInt64 {
		t.Fatalf("expected %d, got %d", maxInt64, decoded.Value)
	}
}

func TestNewServer_NoOptions(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestDefaultCodecsRegistered(t *testing.T) {
	NewServer()
	if encoding.GetCodec("json") == nil {
		t.Fatal("expected json codec to be registered")
	}
	if encoding.GetCodec("protojson") == nil {
		t.Fatal("expected protojson codec to be registered")
	}
}

func TestNewServer_WithNilConfig(t *testing.T) {
	srv := NewServer(WithConfig(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil config")
	}
}

func TestNewServer_WithMiddleware(t *testing.T) {
	srv := NewServer(WithMiddleware(recovery.Recovery()))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithEmptyMiddleware(t *testing.T) {
	srv := NewServer(WithMiddleware())
	if srv == nil {
		t.Fatal("expected non-nil server with empty middleware")
	}
}

func TestNewServer_WithCORS(t *testing.T) {
	corsConf := &corsv1.CORS{
		Enable:         true,
		AllowedOrigins: []string{"*"},
	}
	srv := NewServer(WithCORS(corsConf))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithCORSDisabled(t *testing.T) {
	corsConf := &corsv1.CORS{Enable: false}
	srv := NewServer(WithCORS(corsConf))
	if srv == nil {
		t.Fatal("expected non-nil server with disabled CORS")
	}
}

func TestNewServer_WithNilCORS(t *testing.T) {
	srv := NewServer(WithCORS(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil CORS")
	}
}

func TestNewServer_WithServices(t *testing.T) {
	called := false
	srv := NewServer(WithServices(func(s *khttp.Server) {
		called = true
		_ = s
	}))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if !called {
		t.Fatal("expected registrar to be called")
	}
}

func TestNewServer_WithMultipleServices(t *testing.T) {
	callCount := 0
	srv := NewServer(WithServices(
		func(s *khttp.Server) { callCount++ },
		func(s *khttp.Server) { callCount++ },
		func(s *khttp.Server) { callCount++ },
	))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 registrars called, got %d", callCount)
	}
}

func TestNewServer_WithHealthCheck(t *testing.T) {
	h := health.NewHandler()
	srv := NewServer(WithHealthCheck(h))
	if srv == nil {
		t.Fatal("expected non-nil server with health check")
	}
}

func TestNewServer_WithNilHealthCheck(t *testing.T) {
	srv := NewServer(WithHealthCheck(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil health check")
	}
}

func TestNewServer_WithAdvertiseHost_EndpointUsesAdvertiseHost(t *testing.T) {
	cfg := &corev1.Server_HTTP{
		Listen:    &corev1.Server_Listen{Addr: proto.String("0.0.0.0:0")},
		Advertise: &corev1.Server_Advertise{Host: "host.docker.internal"},
	}

	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if got, want := ep.Host, "host.docker.internal:0"; got != want {
		t.Fatalf("expected host %q, got %q", want, got)
	}
	if got, want := ep.Scheme, "http"; got != want {
		t.Fatalf("expected scheme %q, got %q", want, got)
	}
}

func TestNewServer_WithAdvertiseEndpoint_EndpointUsesExplicitValue(t *testing.T) {
	cfg := &corev1.Server_HTTP{
		Listen:    &corev1.Server_Listen{Addr: proto.String(":0")},
		Advertise: &corev1.Server_Advertise{Endpoint: "https://example.internal:18443?isSecure=true"},
	}

	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	ep, err := srv.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint() error = %v", err)
	}
	if got, want := ep.String(), "https://example.internal:18443?isSecure=true"; got != want {
		t.Fatalf("expected endpoint %q, got %q", want, got)
	}
}

func TestNewServer_APIDocsFromConfig(t *testing.T) {
	document := []byte("openapi: 3.1.0\ninfo: {title: Orders, version: '1'}\npaths: {}\n")
	c := &corev1.Server_HTTP{ApiDocs: &apidocsv1.APIDocs{
		Enable:    true,
		BasePath:  proto.String("/api-docs"),
		Documents: []*apidocsv1.Document{{Source: &apidocsv1.Document_Data{Data: document}}},
	}}
	srv := NewServer(WithConfig(c), WithServices(func(s *khttp.Server) {
		s.HandleFunc("/api-docs-other", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("business"))
		})
	}))
	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/api-docs/", http.StatusOK, ""},
		{"/api-docs/openapi.yaml", http.StatusOK, string(document)},
		{"/api-docs/missing", http.StatusNotFound, ""},
		{"/api-docs-other", http.StatusOK, "business"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if w.Code != tc.status || (tc.body != "" && w.Body.String() != tc.body) {
				t.Fatalf("response = %d %q, want %d %q", w.Code, w.Body.String(), tc.status, tc.body)
			}
		})
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api-docs?view=api", nil))
	location, err := w.Result().Location()
	if err != nil || w.Code < 300 || w.Code >= 400 || location.String() != "api-docs/?view=api" {
		t.Fatalf("redirect = %d %v, error = %v", w.Code, location, err)
	}
}

func TestNewServer_APIDocsDisabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []ServerOption
	}{
		{"absent", nil},
		{"nil", []ServerOption{WithConfig(nil)}},
		{"disabled with invalid path", []ServerOption{WithConfig(&corev1.Server_HTTP{ApiDocs: &apidocsv1.APIDocs{Path: proto.String("missing"), BasePath: proto.String("invalid")}})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := NewServer(tc.opts...)
			for _, path := range []string{"/docs/", "/docs/openapi.yaml"} {
				w := httptest.NewRecorder()
				srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				if w.Code != http.StatusNotFound {
					t.Fatalf("disabled document %s: status = %d", path, w.Code)
				}
			}
		})
	}
}

func TestNewServer_APIDocsConfigurationFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "missing.yaml")
	defer func() {
		err, ok := recover().(error)
		if !ok || !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), file) || !strings.Contains(err.Error(), "server.http.api_docs") {
			t.Fatalf("missing enabled document must fail construction with context: %v", err)
		}
	}()
	NewServer(WithConfig(&corev1.Server_HTTP{ApiDocs: &apidocsv1.APIDocs{Enable: true, Path: proto.String(file)}}))
}

func TestNewServer_APIDocsBootstrapConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		name, docs string
		enabled    bool
	}{
		{"dev does not imply enabled", "", false},
		{"explicitly disabled", "    api_docs:\n      enable: false\n      path: missing.yaml\n", false},
		{"enabled with explicit false sidebar", "    api_docs:\n      enable: true\n      scalar:\n        show_sidebar: false\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "bootstrap.yaml")
			data := fmt.Sprintf("app:\n  env: dev\nserver:\n  http:\n    listen:\n      addr: '127.0.0.1:0'\n%s", tc.docs)
			if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			bc, cfg, err := bootstrapconfig.LoadBootstrap(configPath, "docs.service", false)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cfg.Close() })
			document := []byte("openapi: 3.1.0\ninfo: {title: Generated config, version: '1'}\npaths: {}\n")
			if tc.enabled {
				if err := os.MkdirAll("api/internal/assets", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile("api/internal/assets/openapi.yaml", document, 0o600); err != nil {
					t.Fatal(err)
				}
				scalar := bc.GetServer().GetHttp().GetApiDocs().GetScalar()
				if scalar == nil || scalar.ShowSidebar == nil || *scalar.ShowSidebar {
					t.Fatal("explicit false was lost during config loading/defaults")
				}
			}
			srv := NewServer(WithConfig(bc.GetServer().GetHttp()))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil))
			if tc.enabled {
				if w.Code != http.StatusOK || w.Body.String() != string(document) {
					t.Fatalf("configured document = %d %q", w.Code, w.Body.String())
				}
			} else if w.Code != http.StatusNotFound {
				t.Fatalf("disabled docs = %d", w.Code)
			}
		})
	}
}

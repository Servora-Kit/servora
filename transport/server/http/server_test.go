package http

import (
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/durationpb"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/platform/health"
)

func TestNewServer_NoOptions(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	cfg := &conf.Server_HTTP{
		Listen: &conf.Server_Listen{
			Network: "tcp4",
			Addr:    ":8080",
			Timeout: durationpb.New(30 * time.Second),
		},
	}
	srv := NewServer(WithConfig(cfg))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithNilConfig(t *testing.T) {
	srv := NewServer(WithConfig(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil config")
	}
}

func TestNewServer_WithLogger(t *testing.T) {
	logger := log.DefaultLogger
	srv := NewServer(WithLogger(logger))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithNilLogger(t *testing.T) {
	srv := NewServer(WithLogger(nil))
	if srv == nil {
		t.Fatal("expected non-nil server with nil logger")
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
	corsConf := &conf.CORS{
		Enable:         true,
		AllowedOrigins: []string{"*"},
	}
	srv := NewServer(WithCORS(corsConf))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithCORSDisabled(t *testing.T) {
	corsConf := &conf.CORS{Enable: false}
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

func TestNewServer_FullOptions(t *testing.T) {
	cfg := &conf.Server_HTTP{
		Listen: &conf.Server_Listen{
			Addr:    ":8080",
			Timeout: durationpb.New(10 * time.Second),
		},
	}
	corsConf := &conf.CORS{
		Enable:         true,
		AllowedOrigins: []string{"http://localhost"},
	}
	srv := NewServer(
		WithConfig(cfg),
		WithLogger(log.DefaultLogger),
		WithMiddleware(recovery.Recovery()),
		WithCORS(corsConf),
	)
	if srv == nil {
		t.Fatal("expected non-nil server with full options")
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

func TestNewServer_WithRegistryHost_EndpointUsesRegistryHost(t *testing.T) {
	cfg := &conf.Server_HTTP{
		Listen:   &conf.Server_Listen{Addr: "0.0.0.0:0"},
		Registry: &conf.Server_Registry{Host: "host.docker.internal"},
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

func TestNewServer_WithRegistryEndpoint_EndpointUsesExplicitValue(t *testing.T) {
	cfg := &conf.Server_HTTP{
		Listen:   &conf.Server_Listen{Addr: ":0"},
		Registry: &conf.Server_Registry{Endpoint: "https://example.internal:18443?isSecure=true"},
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

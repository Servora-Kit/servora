package grpc

import (
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

func TestNewServer_NoOptions(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	cfg := &conf.Server_GRPC{
		Network: "tcp4",
		Addr:    ":9000",
		Timeout: durationpb.New(30 * time.Second),
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

func TestNewServer_WithServices(t *testing.T) {
	called := false
	srv := NewServer(WithServices(func(s *kgrpc.Server) {
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
		func(s *kgrpc.Server) { callCount++ },
		func(s *kgrpc.Server) { callCount++ },
		func(s *kgrpc.Server) { callCount++ },
	))
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 registrars called, got %d", callCount)
	}
}

func TestNewServer_FullOptions(t *testing.T) {
	cfg := &conf.Server_GRPC{
		Addr:    ":9000",
		Timeout: durationpb.New(10 * time.Second),
	}
	srv := NewServer(
		WithConfig(cfg),
		WithLogger(log.DefaultLogger),
		WithMiddleware(recovery.Recovery()),
	)
	if srv == nil {
		t.Fatal("expected non-nil server with full options")
	}
}

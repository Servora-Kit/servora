package grpc

import (
	"context"
	"testing"

	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

func TestBuilder_Build(t *testing.T) {
	called := false
	srv, err := NewBuilder().
		WithServices(func(s *kgrpc.Server) {
			called = true
			_ = s
		}).
		Build(context.Background())
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if !called {
		t.Fatal("expected registrar to be called")
	}
}

func TestBuilder_MustBuild(t *testing.T) {
	srv := NewBuilder().MustBuild()
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestBuilder_BuildWithTODOContext(t *testing.T) {
	_, err := NewBuilder().Build(context.TODO())
	if err != nil {
		t.Fatalf("build with TODO context should not fail: %v", err)
	}
}

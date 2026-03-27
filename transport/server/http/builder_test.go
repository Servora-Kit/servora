package http

import (
	"context"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/platform/swagger"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

func TestBuilder_Build(t *testing.T) {
	called := false
	srv, err := NewBuilder().
		WithConfig(&conf.Server_HTTP{}).
		WithServices(func(s *khttp.Server) {
			called = true
			_ = s
		}).
		WithSwagger([]byte(`openapi: "3.0.0"`), swagger.WithTitle("test")).
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

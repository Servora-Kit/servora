package http

import (
	"context"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/runtime"
)

func TestHTTPPlugin_Type(t *testing.T) {
	if (&Plugin{}).Type() != Type {
		t.Fatal("unexpected type")
	}
}

func TestHTTPPlugin_BuildReturnsServer(t *testing.T) {
	p := &Plugin{}
	srv, err := p.Build(context.Background(), runtime.ServerBuildInput{
		Config: &conf.Server_HTTP{Listen: &conf.Server_Listen{Addr: ":0"}},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

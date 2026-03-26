package sse

import (
	"context"
	"testing"

	"github.com/Servora-Kit/servora/transport/runtime"
)

func TestSSEPlugin_Type(t *testing.T) {
	if (&Plugin{}).Type() != Type {
		t.Fatal("unexpected type")
	}
}

func TestSSEPlugin_BuildReturnsNoopServer(t *testing.T) {
	p := &Plugin{}
	srv, err := p.Build(context.Background(), runtime.ServerBuildInput{})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

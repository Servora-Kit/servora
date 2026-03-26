package runtime

import (
	"context"
	"testing"
)

func TestGraph_BuildsConfiguredPlugins(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterServer(&fakeServerPlugin{typ: "grpc"}); err != nil {
		t.Fatalf("register server plugin: %v", err)
	}
	if err := r.RegisterClient(&fakeClientPlugin{typ: "grpc"}); err != nil {
		t.Fatalf("register client plugin: %v", err)
	}

	g := NewGraph(r)
	out, err := g.Build(context.Background(), GraphInput{
		Servers: []ServerNode{{Type: "grpc"}},
		Clients: []ClientNode{{Type: "grpc"}},
	})
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	if len(out.Servers) != 1 {
		t.Fatalf("expected one server, got %d", len(out.Servers))
	}
	if len(out.Clients) != 1 {
		t.Fatalf("expected one client, got %d", len(out.Clients))
	}
}

func TestGraph_UnknownPluginFailsFast(t *testing.T) {
	g := NewGraph(NewRegistry())
	_, err := g.Build(context.Background(), GraphInput{
		Servers: []ServerNode{{Type: "grpc"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown plugin")
	}
}

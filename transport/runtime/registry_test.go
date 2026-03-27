package runtime

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestRegistry_RegisterAndResolveServerPlugin(t *testing.T) {
	r := NewRegistry()
	p := &fakeServerPlugin{typ: "grpc"}

	if err := r.RegisterServer(p); err != nil {
		t.Fatalf("register server: %v", err)
	}

	got, ok := r.Server("grpc")
	if !ok || got.Type() != "grpc" {
		t.Fatalf("expected grpc plugin, got ok=%v type=%v", ok, got)
	}
}

func TestRegistry_DuplicateServerPluginRejected(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterServer(&fakeServerPlugin{typ: "grpc"})

	err := r.RegisterServer(&fakeServerPlugin{typ: "grpc"})
	if !errors.Is(err, ErrPluginAlreadyRegistered) {
		t.Fatalf("expected ErrPluginAlreadyRegistered, got %v", err)
	}
}

func TestRegistry_RegisterAndResolveClientPlugin(t *testing.T) {
	r := NewRegistry()
	p := &fakeClientPlugin{typ: "http"}

	if err := r.RegisterClient(p); err != nil {
		t.Fatalf("register client: %v", err)
	}

	got, ok := r.Client("http")
	if !ok || got.Type() != "http" {
		t.Fatalf("expected http plugin, got ok=%v type=%v", ok, got)
	}
}

func TestRegistry_DuplicateClientPluginRejected(t *testing.T) {
	r := NewRegistry()
	_ = r.RegisterClient(&fakeClientPlugin{typ: "grpc"})

	err := r.RegisterClient(&fakeClientPlugin{typ: "grpc"})
	if !errors.Is(err, ErrPluginAlreadyRegistered) {
		t.Fatalf("expected ErrPluginAlreadyRegistered, got %v", err)
	}
}

type fakeServerPlugin struct {
	typ string
}

func (f *fakeServerPlugin) Type() string { return f.typ }

func (f *fakeServerPlugin) Build(context.Context, ServerBuildInput) (Server, error) {
	return fakeServer{}, nil
}

type fakeServer struct{}

func (fakeServer) Start(context.Context) error { return nil }
func (fakeServer) Stop(context.Context) error  { return nil }
func (fakeServer) Endpoint() (*url.URL, error) { return nil, nil }

type fakeClientPlugin struct {
	typ string
}

func (f *fakeClientPlugin) Type() string { return f.typ }

func (f *fakeClientPlugin) Build(context.Context, ClientBuildInput) (ClientFactory, error) {
	return fakeClientFactory{}, nil
}

type fakeClientFactory struct{}

func (fakeClientFactory) Dial(context.Context, ClientDialInput) (Connection, error) {
	return fakeConn{}, nil
}

type fakeConn struct{}

func (fakeConn) Value() any      { return nil }
func (fakeConn) Close() error    { return nil }
func (fakeConn) IsHealthy() bool { return true }

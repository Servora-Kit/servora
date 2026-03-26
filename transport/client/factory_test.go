package client

import (
	"context"
	"errors"
	"testing"

	"github.com/Servora-Kit/servora/transport/runtime"
)

func TestNewClient_WithCustomPlugin(t *testing.T) {
	c, err := NewClient(nil, nil, nil, nil,
		WithoutBuiltinPlugins(),
		WithPlugins(&fakePlugin{typ: "fake"}),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	conn, err := c.CreateConn(context.Background(), ConnType("fake"), "fake.service")
	if err != nil {
		t.Fatalf("create conn: %v", err)
	}
	if got, want := conn.GetType(), ConnType("fake"); got != want {
		t.Fatalf("conn type = %q, want %q", got, want)
	}
	if got, ok := conn.Value().(string); !ok || got != "fake:fake.service" {
		t.Fatalf("conn value = %#v, want %q", conn.Value(), "fake:fake.service")
	}
}

func TestCreateConn_UnknownType(t *testing.T) {
	c, err := NewClient(nil, nil, nil, nil, WithoutBuiltinPlugins())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = c.CreateConn(context.Background(), ConnType("unknown"), "svc")
	if !errors.Is(err, runtime.ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestGetConnValue(t *testing.T) {
	c, err := NewClient(nil, nil, nil, nil,
		WithoutBuiltinPlugins(),
		WithPlugins(&fakePlugin{typ: "fake"}),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	got, err := GetConnValue[string](context.Background(), c, ConnType("fake"), "svc")
	if err != nil {
		t.Fatalf("get conn value: %v", err)
	}
	if got != "fake:svc" {
		t.Fatalf("value = %q, want %q", got, "fake:svc")
	}
}

type fakePlugin struct {
	typ string
}

func (p *fakePlugin) Type() string { return p.typ }

func (p *fakePlugin) Build(context.Context, runtime.ClientBuildInput) (runtime.ClientFactory, error) {
	return fakeFactory{}, nil
}

type fakeFactory struct{}

func (fakeFactory) CreateConn(_ context.Context, serviceName string) (runtime.Connection, error) {
	return fakeConn{v: "fake:" + serviceName}, nil
}

type fakeConn struct {
	v string
}

func (f fakeConn) Value() any      { return f.v }
func (f fakeConn) Close() error    { return nil }
func (f fakeConn) IsHealthy() bool { return true }

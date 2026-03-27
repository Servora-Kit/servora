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

	conn, err := c.Dial(context.Background(), runtime.ClientDialInput{
		Protocol: "fake",
		Target:   "fake.service",
	})
	if err != nil {
		t.Fatalf("dial conn: %v", err)
	}
	if got, want := conn.GetProtocol(), "fake"; got != want {
		t.Fatalf("conn protocol = %q, want %q", got, want)
	}
	if got, ok := conn.Value().(string); !ok || got != "fake:fake.service" {
		t.Fatalf("conn value = %#v, want %q", conn.Value(), "fake:fake.service")
	}
}

func TestDial_UnknownType(t *testing.T) {
	c, err := NewClient(nil, nil, nil, nil, WithoutBuiltinPlugins())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = c.Dial(context.Background(), runtime.ClientDialInput{
		Protocol: "unknown",
		Target:   "svc",
	})
	if !errors.Is(err, runtime.ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got %v", err)
	}
}

func TestGetValue(t *testing.T) {
	c, err := NewClient(nil, nil, nil, nil,
		WithoutBuiltinPlugins(),
		WithPlugins(&fakePlugin{typ: "fake"}),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	got, err := GetValue[string](context.Background(), c, runtime.ClientDialInput{
		Protocol: "fake",
		Target:   "svc",
	})
	if err != nil {
		t.Fatalf("get dial value: %v", err)
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

func (fakeFactory) Dial(_ context.Context, in runtime.ClientDialInput) (runtime.Connection, error) {
	return fakeConn{v: "fake:" + in.Target}, nil
}

type fakeConn struct {
	v string
}

func (f fakeConn) Value() any      { return f.v }
func (f fakeConn) Close() error    { return nil }
func (f fakeConn) IsHealthy() bool { return true }

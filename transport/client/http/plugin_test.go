package http

import (
	"context"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/runtime"
)

func TestPlugin_Type(t *testing.T) {
	if (&Plugin{}).Type() != "http" {
		t.Fatal("unexpected type")
	}
}

func TestPlugin_BuildFactory(t *testing.T) {
	f, err := (&Plugin{}).Build(context.Background(), runtime.ClientBuildInput{
		Data: &conf.Data{
			Client: &conf.Data_Client{},
		},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil factory")
	}
}

func TestBuildHTTPClientConfigIndex(t *testing.T) {
	dataCfg := &conf.Data{
		Client: &conf.Data_Client{
			Http: []*conf.Data_Client_HTTP{
				nil,
				{ServiceName: ""},
				{ServiceName: " master ", Endpoint: "http://first"},
				{ServiceName: "master", Endpoint: "http://second"},
				{ServiceName: "worker", Endpoint: "http://worker"},
			},
		},
	}

	index := BuildClientConfigIndex(dataCfg)
	if len(index) != 2 {
		t.Fatalf("expected 2 indexed services, got %d", len(index))
	}

	if got := index["master"]; got == nil || got.GetEndpoint() != "http://second" {
		t.Fatalf("expected latest master config to win, got %#v", got)
	}

	if got := index["worker"]; got == nil || got.GetEndpoint() != "http://worker" {
		t.Fatalf("expected worker config to be indexed, got %#v", got)
	}
}

func TestPlugin_DialWithDirectTarget(t *testing.T) {
	f, err := (&Plugin{}).Build(context.Background(), runtime.ClientBuildInput{
		Data: &conf.Data{
			Client: &conf.Data_Client{},
		},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	conn, err := f.Dial(context.Background(), runtime.ClientDialInput{
		Protocol: "http",
		Target:   "http://127.0.0.1:8080",
	})
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if conn == nil || !conn.IsHealthy() {
		t.Fatal("expected healthy connection")
	}
}

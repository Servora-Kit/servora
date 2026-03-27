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
			Services: []*conf.Data_Client_Service{
				nil,
				{
					Name: " master ",
					Endpoints: []*conf.Data_Client_Endpoint{
						nil,
						{Protocol: "http", Endpoint: "http://first"},
						{Protocol: "grpc", Endpoint: "grpc://master"},
					},
				},
				{
					Name: "worker",
					Endpoints: []*conf.Data_Client_Endpoint{
						{Protocol: "http", Endpoint: "http://worker"},
					},
				},
			},
		},
	}

	index, err := BuildClientConfigIndex(dataCfg)
	if err != nil {
		t.Fatalf("build index failed: %v", err)
	}
	if len(index) != 2 {
		t.Fatalf("expected 2 indexed services, got %d", len(index))
	}

	if got := index["master"]; got == nil || got.GetEndpoint() != "http://first" {
		t.Fatalf("expected master http config to be indexed, got %#v", got)
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

func TestResolveDefaultHTTPEndpoint(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "direct http url",
			target: "http://127.0.0.1:8080",
			want:   "http://127.0.0.1:8080",
		},
		{
			name:   "direct https url",
			target: "https://api.example.com",
			want:   "https://api.example.com",
		},
		{
			name:   "service name falls back to discovery",
			target: "worker.service",
			want:   "discovery:///worker.service",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDefaultHTTPEndpoint(tc.target); got != tc.want {
				t.Fatalf("resolveDefaultHTTPEndpoint(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}

func TestPlugin_BuildFactory_DuplicateServiceProtocol(t *testing.T) {
	_, err := (&Plugin{}).Build(context.Background(), runtime.ClientBuildInput{
		Data: &conf.Data{
			Client: &conf.Data_Client{
				Services: []*conf.Data_Client_Service{
					{
						Name: "master",
						Endpoints: []*conf.Data_Client_Endpoint{
							{Protocol: "http", Endpoint: "http://a"},
							{Protocol: "http", Endpoint: "http://b"},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate config error")
	}
}

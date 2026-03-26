package client

import (
	"testing"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestBuildGRPCClientConfigIndex(t *testing.T) {
	dataCfg := &conf.Data{
		Client: &conf.Data_Client{
			Grpc: []*conf.Data_Client_GRPC{
				nil,
				{ServiceName: ""},
				{ServiceName: " user ", Endpoint: "grpc://first"},
				{ServiceName: "user", Endpoint: "grpc://second"},
				{ServiceName: "auth", Endpoint: "grpc://auth"},
			},
		},
	}

	index := initGRPCClients(dataCfg)
	if len(index) != 2 {
		t.Fatalf("expected 2 indexed services, got %d", len(index))
	}

	if got := index["user"]; got == nil || got.GetEndpoint() != "grpc://second" {
		t.Fatalf("expected latest user config to win, got %#v", got)
	}

	if got := index["auth"]; got == nil || got.GetEndpoint() != "grpc://auth" {
		t.Fatalf("expected auth config to be indexed, got %#v", got)
	}

	if _, ok := index[""]; ok {
		t.Fatal("expected blank service name to be skipped")
	}
}

func TestResolveGRPCConnectionConfig(t *testing.T) {
	defaultEndpoint := "discovery:///user.service"
	defaultTimeout := 5 * time.Second

	endpoint, timeout, configured := resolveGRPCConnectionConfig(
		"user.service",
		map[string]*conf.Data_Client_GRPC{
			"user.service": {
				ServiceName: "user.service",
				Endpoint:    "dns:///user.internal:9000",
				Timeout:     durationpb.New(12 * time.Second),
			},
		},
		defaultEndpoint,
		defaultTimeout,
	)

	if endpoint != "dns:///user.internal:9000" {
		t.Fatalf("expected configured endpoint, got %q", endpoint)
	}
	if timeout != 12*time.Second {
		t.Fatalf("expected configured timeout, got %s", timeout)
	}
	if !configured {
		t.Fatal("expected config to be marked as configured")
	}

	endpoint, timeout, configured = resolveGRPCConnectionConfig(
		"missing.service",
		map[string]*conf.Data_Client_GRPC{
			"user.service": {ServiceName: "user.service"},
		},
		defaultEndpoint,
		defaultTimeout,
	)

	if endpoint != defaultEndpoint {
		t.Fatalf("expected default endpoint, got %q", endpoint)
	}
	if timeout != defaultTimeout {
		t.Fatalf("expected default timeout, got %s", timeout)
	}
	if configured {
		t.Fatal("expected missing service to use defaults")
	}
}

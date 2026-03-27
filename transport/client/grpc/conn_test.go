package grpc

import (
	"context"
	"testing"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestBuildGRPCClientConfigIndex(t *testing.T) {
	dataCfg := &conf.Data{
		Client: &conf.Data_Client{
			Services: []*conf.Data_Client_Service{
				nil,
				{
					Name: " user ",
					Endpoints: []*conf.Data_Client_Endpoint{
						nil,
						{Protocol: "grpc", Endpoint: "grpc://first"},
						{Protocol: "http", Endpoint: "http://user"},
					},
				},
				{
					Name: "auth",
					Endpoints: []*conf.Data_Client_Endpoint{
						{Protocol: "grpc", Endpoint: "grpc://auth"},
					},
				},
			},
		},
	}

	index, err := BuildClientConfigIndex(dataCfg)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	if len(index) != 2 {
		t.Fatalf("expected 2 indexed services, got %d", len(index))
	}

	if got := index["user"]; got == nil || got.GetEndpoint() != "grpc://first" {
		t.Fatalf("expected user grpc config to be indexed, got %#v", got)
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

	endpoint, timeout, tlsCfg, configured := resolveConnectionConfig(
		"user.service",
		map[string]*conf.Data_Client_Endpoint{
			"user.service": {
				Protocol: "grpc",
				Endpoint: "dns:///user.internal:9000",
				Timeout:  durationpb.New(12 * time.Second),
				Tls: &conf.TLSConfig{
					Enable: true,
				},
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
	if tlsCfg == nil || !tlsCfg.GetEnable() {
		t.Fatal("expected tls config to be returned")
	}
	if !configured {
		t.Fatal("expected config to be marked as configured")
	}

	endpoint, timeout, tlsCfg, configured = resolveConnectionConfig(
		"missing.service",
		map[string]*conf.Data_Client_Endpoint{
			"user.service": {Protocol: "grpc"},
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
	if tlsCfg != nil {
		t.Fatal("expected missing service to have nil tls config")
	}
	if configured {
		t.Fatal("expected missing service to use defaults")
	}
}

func TestDialConnection_InvalidTLSConfig(t *testing.T) {
	_, err := dialConnection(
		context.Background(),
		[]kgrpc.ClientOption{
			kgrpc.WithEndpoint("discovery:///user.service"),
			kgrpc.WithTimeout(100 * time.Millisecond),
		},
		&conf.TLSConfig{
			Enable:   true,
			CertPath: "/tmp/client.crt",
		},
	)
	if err == nil {
		t.Fatal("expected invalid tls config to return error")
	}
}

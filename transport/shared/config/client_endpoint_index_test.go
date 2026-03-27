package config

import (
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

func TestBuildClientEndpointIndex_FilterByProtocol(t *testing.T) {
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

	index, err := BuildClientEndpointIndex(dataCfg, "grpc")
	if err != nil {
		t.Fatalf("build endpoint index: %v", err)
	}
	if len(index) != 2 {
		t.Fatalf("index len = %d, want 2", len(index))
	}
	if got := index["user"]; got == nil || got.GetEndpoint() != "grpc://first" {
		t.Fatalf("user endpoint mismatch: %#v", got)
	}
	if got := index["auth"]; got == nil || got.GetEndpoint() != "grpc://auth" {
		t.Fatalf("auth endpoint mismatch: %#v", got)
	}
}

func TestBuildClientEndpointIndex_EmptyInputs(t *testing.T) {
	if got, err := BuildClientEndpointIndex(nil, "grpc"); err != nil || got != nil {
		t.Fatalf("expected nil for nil data, got %#v", got)
	}
	if got, err := BuildClientEndpointIndex(&conf.Data{Client: &conf.Data_Client{}}, "grpc"); err != nil || got != nil {
		t.Fatalf("expected nil for empty services, got %#v", got)
	}
	if _, err := BuildClientEndpointIndex(&conf.Data{Client: &conf.Data_Client{}}, " "); err == nil {
		t.Fatal("expected error for blank protocol")
	}
}

func TestBuildClientEndpointIndex_EmptyServiceName(t *testing.T) {
	dataCfg := &conf.Data{
		Client: &conf.Data_Client{
			Services: []*conf.Data_Client_Service{
				{Name: " ", Endpoints: []*conf.Data_Client_Endpoint{{Protocol: "grpc", Endpoint: "grpc://a"}}},
			},
		},
	}

	if _, err := BuildClientEndpointIndex(dataCfg, "grpc"); err == nil {
		t.Fatal("expected empty service name error")
	}
}

func TestBuildClientEndpointIndex_DuplicateServiceProtocol(t *testing.T) {
	dataCfg := &conf.Data{
		Client: &conf.Data_Client{
			Services: []*conf.Data_Client_Service{
				{
					Name: "user",
					Endpoints: []*conf.Data_Client_Endpoint{
						{Protocol: "grpc", Endpoint: "grpc://a"},
						{Protocol: "grpc", Endpoint: "grpc://b"},
					},
				},
			},
		},
	}

	if _, err := BuildClientEndpointIndex(dataCfg, "grpc"); err == nil {
		t.Fatal("expected duplicate endpoint error")
	}
}

func TestBuildClientEndpointIndex_EmptyEndpointProtocol(t *testing.T) {
	dataCfg := &conf.Data{
		Client: &conf.Data_Client{
			Services: []*conf.Data_Client_Service{
				{
					Name: "user",
					Endpoints: []*conf.Data_Client_Endpoint{
						{Protocol: "", Endpoint: "grpc://a"},
					},
				},
			},
		},
	}

	if _, err := BuildClientEndpointIndex(dataCfg, "grpc"); err == nil {
		t.Fatal("expected empty protocol error")
	}
}

package endpoint

import (
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestIndexByProtocol_FilterByProtocol(t *testing.T) {
	dataCfg := &corev1.Data{
		Client: &corev1.Data_Client{
			Services: []*corev1.Data_Client_Service{
				nil,
				{
					Name: " user ",
					Endpoints: []*corev1.Data_Client_Endpoint{
						nil,
						{Protocol: "grpc", Endpoint: "grpc://first"},
						{Protocol: "http", Endpoint: "http://user"},
					},
				},
				{
					Name: "auth",
					Endpoints: []*corev1.Data_Client_Endpoint{
						{Protocol: "grpc", Endpoint: "grpc://auth"},
					},
				},
			},
		},
	}

	index, err := IndexByProtocol(dataCfg, "grpc")
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

func TestIndexByProtocol_EmptyInputs(t *testing.T) {
	if got, err := IndexByProtocol(nil, "grpc"); err != nil || got != nil {
		t.Fatalf("expected nil for nil data, got %#v", got)
	}
	if got, err := IndexByProtocol(&corev1.Data{Client: &corev1.Data_Client{}}, "grpc"); err != nil || got != nil {
		t.Fatalf("expected nil for empty services, got %#v", got)
	}
	if _, err := IndexByProtocol(&corev1.Data{Client: &corev1.Data_Client{}}, " "); err == nil {
		t.Fatal("expected error for blank protocol")
	}
}

func TestIndexByProtocol_EmptyServiceName(t *testing.T) {
	dataCfg := &corev1.Data{
		Client: &corev1.Data_Client{
			Services: []*corev1.Data_Client_Service{
				{Name: " ", Endpoints: []*corev1.Data_Client_Endpoint{{Protocol: "grpc", Endpoint: "grpc://a"}}},
			},
		},
	}

	if _, err := IndexByProtocol(dataCfg, "grpc"); err == nil {
		t.Fatal("expected empty service name error")
	}
}

func TestIndexByProtocol_DuplicateServiceProtocol(t *testing.T) {
	dataCfg := &corev1.Data{
		Client: &corev1.Data_Client{
			Services: []*corev1.Data_Client_Service{
				{
					Name: "user",
					Endpoints: []*corev1.Data_Client_Endpoint{
						{Protocol: "grpc", Endpoint: "grpc://a"},
						{Protocol: "grpc", Endpoint: "grpc://b"},
					},
				},
			},
		},
	}

	if _, err := IndexByProtocol(dataCfg, "grpc"); err == nil {
		t.Fatal("expected duplicate endpoint error")
	}
}

func TestIndexByProtocol_EmptyEndpointProtocol(t *testing.T) {
	dataCfg := &corev1.Data{
		Client: &corev1.Data_Client{
			Services: []*corev1.Data_Client_Service{
				{
					Name: "user",
					Endpoints: []*corev1.Data_Client_Endpoint{
						{Protocol: "", Endpoint: "grpc://a"},
					},
				},
			},
		},
	}

	if _, err := IndexByProtocol(dataCfg, "grpc"); err == nil {
		t.Fatal("expected empty protocol error")
	}
}

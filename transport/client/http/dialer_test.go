package http

import (
	"testing"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
)

func TestBuildHTTPClientConfigIndex(t *testing.T) {
	dataCfg := &corev1.Data{
		Client: &corev1.Data_Client{
			Services: []*corev1.Data_Client_Service{
				nil,
				{
					Name: " master ",
					Endpoints: []*corev1.Data_Client_Endpoint{
						nil,
						{Protocol: "http", Endpoint: "http://first"},
						{Protocol: "grpc", Endpoint: "grpc://master"},
					},
				},
				{
					Name: "worker",
					Endpoints: []*corev1.Data_Client_Endpoint{
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

func TestDialer_DialWithDirectTarget(t *testing.T) {
	d := NewDialer()
	client, err := d.Dial(t.Context(), "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	_ = client.Close()
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

package registry

import (
	"net/url"
	"testing"
)

func TestResolveRegistryEndpoint_UsesExplicitEndpoint(t *testing.T) {
	ep, err := ResolveRegistryEndpoint(RegistryEndpointInput{
		Scheme:   "grpcs",
		BindAddr: "0.0.0.0:8011",
		Endpoint: "grpcs://svc.internal:8011?isSecure=true",
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if got, want := ep.String(), "grpcs://svc.internal:8011?isSecure=true"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestResolveRegistryEndpoint_UsesHostAndBindPort(t *testing.T) {
	q := url.Values{}
	q.Set("isSecure", "true")
	ep, err := ResolveRegistryEndpoint(RegistryEndpointInput{
		Scheme:   "grpcs",
		BindAddr: "0.0.0.0:8011",
		Host:     "192.168.1.10",
		Query:    q,
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if got, want := ep.String(), "grpcs://192.168.1.10:8011?isSecure=true"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestResolveRegistryEndpoint_NoHostReturnsNil(t *testing.T) {
	ep, err := ResolveRegistryEndpoint(RegistryEndpointInput{
		Scheme:   "grpc",
		BindAddr: "0.0.0.0:8011",
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if ep != nil {
		t.Fatalf("expected nil endpoint, got %v", ep)
	}
}

func TestResolveRegistryEndpoint_NoQueryParams(t *testing.T) {
	ep, err := ResolveRegistryEndpoint(RegistryEndpointInput{
		Scheme:   "ws",
		BindAddr: "0.0.0.0:9000",
		Host:     "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if got, want := ep.String(), "ws://10.0.0.1:9000"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

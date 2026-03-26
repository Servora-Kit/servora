package endpoint

import "testing"

func TestResolveRegistryEndpoint_UsesExplicitEndpoint(t *testing.T) {
	ep, err := ResolveRegistryEndpoint("grpc", "0.0.0.0:8011", "grpcs://svc.internal:8011?isSecure=true", "", true)
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if got, want := ep.String(), "grpcs://svc.internal:8011?isSecure=true"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestResolveRegistryEndpoint_UsesHostAndBindPort(t *testing.T) {
	ep, err := ResolveRegistryEndpoint("grpc", "0.0.0.0:8011", "", "192.168.1.10", true)
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if got, want := ep.String(), "grpcs://192.168.1.10:8011?isSecure=true"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestResolveRegistryEndpoint_NoHostReturnsNil(t *testing.T) {
	ep, err := ResolveRegistryEndpoint("grpc", "0.0.0.0:8011", "", "", false)
	if err != nil {
		t.Fatalf("resolve endpoint: %v", err)
	}
	if ep != nil {
		t.Fatalf("expected nil endpoint, got %v", ep)
	}
}

package builtin

import "testing"

func TestNewRegistry_ContainsBuiltinPlugins(t *testing.T) {
	r, err := NewRegistry()
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	if _, ok := r.Server("grpc"); !ok {
		t.Fatal("missing grpc server plugin")
	}
	if _, ok := r.Server("http"); !ok {
		t.Fatal("missing http server plugin")
	}
	if _, ok := r.Server("sse"); !ok {
		t.Fatal("missing sse server plugin")
	}
	if _, ok := r.Client("grpc"); !ok {
		t.Fatal("missing grpc client plugin")
	}
	if _, ok := r.Client("http"); !ok {
		t.Fatal("missing http client plugin")
	}
}

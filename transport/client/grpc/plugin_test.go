package grpc

import (
	"context"
	"testing"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/runtime"
)

func TestPlugin_Type(t *testing.T) {
	if (&Plugin{}).Type() != "grpc" {
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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
	"github.com/Servora-Kit/servora/cmd/internal/plugintest"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestCrossPackageDefaultsCompileAndRun(t *testing.T) {
	t.Parallel()
	parent := &descriptorpb.FileDescriptorProto{
		Name: new("fixture.proto"), Package: new("fixture"), Syntax: new("proto3"),
		Options:    &descriptorpb.FileOptions{GoPackage: new("example.com/fixture;fixture")},
		Dependency: []string{"servora/transport/http/cors/v1/config.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: new("Parent"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name: new("cors"), Number: proto.Int32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: new(".servora.transport.http.cors.v1.CORS"),
			}},
		}},
	}
	files := plugintest.DescriptorClosure(corsv1.File_servora_transport_http_cors_v1_config_proto)
	plugin, err := (protogen.Options{}).New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{parent.GetName()},
		Parameter:      new("paths=source_relative"),
		ProtoFile:      append(files, parent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(plugin); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	generated := plugintest.ResponseFiles(plugin)
	generated["go.mod"] = "module example.com/fixture\n\ngo 1.27.0\n\nrequire github.com/Servora-Kit/servora v0.0.0\nreplace github.com/Servora-Kit/servora => " + repoRoot + "\n"
	// Companion methods must compile against a parent with a field in another Go package.
	generated["fixture_test.go"] = `package fixture
import (
 "testing"
 corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
)
type Parent struct { Cors *corsv1.CORS }
func TestDefaults(t *testing.T) {
 var p Parent
 p.ApplyDefaults()
 if p.Cors == nil || len(p.Cors.AllowedOrigins) != 1 || p.Cors.AllowedOrigins[0] != "*" {
  t.Fatalf("nested defaults not applied: %v", p.Cors)
 }
 p.Cors.AllowedOrigins = []string{"https://example.com"}
 p.ApplyDefaults()
 if len(p.Cors.AllowedOrigins) != 1 || p.Cors.AllowedOrigins[0] != "https://example.com" {
  t.Fatalf("explicit nested value overwritten: %v", p.Cors)
 }
}
`
	for name, content := range generated {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated consumer failed: %v\n%s", err, output)
	}
}

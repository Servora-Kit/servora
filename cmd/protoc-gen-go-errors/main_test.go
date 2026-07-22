package main

import (
	"strings"
	"testing"

	errorsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/errors/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGenerateErrors(t *testing.T) {
	t.Parallel()

	enumOptions := &descriptorpb.EnumOptions{}
	proto.SetExtension(enumOptions, errorsv1.E_DefaultCode, int32(500))
	invalidOptions := &descriptorpb.EnumValueOptions{}
	proto.SetExtension(invalidOptions, errorsv1.E_Code, int32(400))

	target := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("example/v1/errors.proto"),
		Package:    proto.String("example.v1"),
		Dependency: []string{"servora/errors/v1/errors.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("example.com/gen/example/v1;examplev1"),
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name:    proto.String("ErrorReason"),
				Options: enumOptions,
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("ERROR_REASON_UNSPECIFIED"), Number: proto.Int32(0)},
					{Name: proto.String("ERROR_REASON_INVALID_INPUT"), Number: proto.Int32(1), Options: invalidOptions},
					{Name: proto.String("ERROR_REASON_INTERNAL"), Number: proto.Int32(2)},
				},
			},
		},
	}

	generated := runGenerator(t, target)
	content, ok := generated["example/v1/errors_errors.pb.go"]
	if !ok {
		t.Fatalf("generated files = %v, want example/v1/errors_errors.pb.go", generated)
	}

	assertContains(t, content, `"github.com/go-kratos/kratos/v3/errors"`)
	assertContains(t, content, "func ErrorErrorReasonInvalidInput(")
	assertContains(t, content, "errors.New(400, ErrorReason_ERROR_REASON_INVALID_INPUT.String()")
	assertContains(t, content, "errors.New(500, ErrorReason_ERROR_REASON_INTERNAL.String()")
	if strings.Contains(content, "go-kratos/kratos/v2") {
		t.Fatalf("generated content imports Kratos v2:\n%s", content)
	}
}

func TestGenerateSkipsUnannotatedEnums(t *testing.T) {
	t.Parallel()

	target := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("example/v1/plain.proto"),
		Package: proto.String("example.v1"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("example.com/gen/example/v1;examplev1"),
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Plain"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("PLAIN_UNSPECIFIED"), Number: proto.Int32(0)},
				},
			},
		},
	}

	if generated := runGenerator(t, target); len(generated) != 0 {
		t.Fatalf("generated files = %v, want none", generated)
	}
}

func runGenerator(t *testing.T, target *descriptorpb.FileDescriptorProto) map[string]string {
	t.Helper()

	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{target.GetName()},
		Parameter:      proto.String("paths=source_relative"),
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto),
			protodesc.ToFileDescriptorProto(errorsv1.File_servora_errors_v1_errors_proto),
			target,
		},
	}
	plugin, err := protogen.Options{}.New(request)
	if err != nil {
		t.Fatalf("protogen.Options.New: %v", err)
	}
	if err := generate(plugin); err != nil {
		t.Fatalf("generate: %v", err)
	}

	result := make(map[string]string, len(plugin.Response().File))
	for _, file := range plugin.Response().File {
		result[file.GetName()] = file.GetContent()
	}
	return result
}

func assertContains(t *testing.T, value, substring string) {
	t.Helper()
	if !strings.Contains(value, substring) {
		t.Fatalf("generated content does not contain %q:\n%s", substring, value)
	}
}

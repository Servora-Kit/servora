package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGenerateUsesProtoJSONTypesFor64BitIntegers(t *testing.T) {
	output := generateContractFixture(t)

	for _, declaration := range []string{
		"int64Value: string | undefined;",
		"uint64Value: string | undefined;",
		"sint64Value: string | undefined;",
		"fixed64Value: string | undefined;",
		"sfixed64Value: string | undefined;",
		"optionalInt64?: string;",
		"repeatedInt64: string[] | undefined;",
		"uint64ByKey: { [key: string]: string } | undefined;",
		"type wellKnownInt64Value = null | string;",
		"type wellKnownUInt64Value = null | string;",
	} {
		assert.Contains(t, output, declaration)
	}

	for _, declaration := range []string{
		"int32Value: number | undefined;",
		"uint32Value: number | undefined;",
		"floatValue: number | undefined;",
		"doubleValue: number | undefined;",
		"type wellKnownInt32Value = null | number;",
		"type wellKnownUInt32Value = null | number;",
	} {
		assert.Contains(t, output, declaration)
	}
}

func TestGenerateEncodesHTTPPathVariablesByTemplateShape(t *testing.T) {
	output := generateContractFixture(t)

	assert.Contains(t, output, "const path = `v1/${encodeMultiSegmentPath(request.name)}`;")
	assert.Contains(t, output, "const path = `v1/users/${encodePathSegment(request.id)}`;")
	assert.Contains(t, output, "function encodePathSegment(value: unknown): string {")
	assert.Contains(t, output, "function encodeMultiSegmentPath(value: unknown): string {")
	assert.NotContains(t, output, "const path = `v1/${request.name}`;")
	assert.NotContains(t, output, "const path = `v1/users/${request.id}`;")
}

func generateContractFixture(t *testing.T) string {
	t.Helper()

	file := contractFixtureDescriptor()
	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{file.GetName()},
		ProtoFile: descriptorClosure(
			annotations.File_google_api_annotations_proto,
			wrapperspb.File_google_protobuf_wrappers_proto,
		),
	}
	request.ProtoFile = append(request.ProtoFile, file)

	response, err := Generate(request)
	require.NoError(t, err)
	require.Len(t, response.File, 1)
	assert.Equal(t, "test/v1/index.ts", response.File[0].GetName())
	return response.File[0].GetContent()
}

func contractFixtureDescriptor() *descriptorpb.FileDescriptorProto {
	payload := &descriptorpb.DescriptorProto{
		Name: proto.String("Payload"),
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("_optional_int64")},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{
				Name:    proto.String("Uint64ByKeyEntry"),
				Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				Field: []*descriptorpb.FieldDescriptorProto{
					scalarField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
					scalarField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_UINT64),
				},
			},
		},
		Field: []*descriptorpb.FieldDescriptorProto{
			scalarField("int32_value", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32),
			scalarField("uint32_value", 2, descriptorpb.FieldDescriptorProto_TYPE_UINT32),
			scalarField("float_value", 3, descriptorpb.FieldDescriptorProto_TYPE_FLOAT),
			scalarField("double_value", 4, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE),
			scalarField("int64_value", 5, descriptorpb.FieldDescriptorProto_TYPE_INT64),
			scalarField("uint64_value", 6, descriptorpb.FieldDescriptorProto_TYPE_UINT64),
			scalarField("sint64_value", 7, descriptorpb.FieldDescriptorProto_TYPE_SINT64),
			scalarField("fixed64_value", 8, descriptorpb.FieldDescriptorProto_TYPE_FIXED64),
			scalarField("sfixed64_value", 9, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64),
			{
				Name:           proto.String("optional_int64"),
				Number:         proto.Int32(10),
				Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:           descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				OneofIndex:     proto.Int32(0),
				Proto3Optional: proto.Bool(true),
			},
			{
				Name:   proto.String("repeated_int64"),
				Number: proto.Int32(11),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
			},
			{
				Name:     proto.String("uint64_by_key"),
				Number:   proto.Int32(12),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".test.v1.Payload.Uint64ByKeyEntry"),
			},
			messageField("int64_wrapper", 13, ".google.protobuf.Int64Value"),
			messageField("uint64_wrapper", 14, ".google.protobuf.UInt64Value"),
			messageField("int32_wrapper", 15, ".google.protobuf.Int32Value"),
			messageField("uint32_wrapper", 16, ".google.protobuf.UInt32Value"),
		},
	}

	request := &descriptorpb.DescriptorProto{
		Name: proto.String("GetRequest"),
		Field: []*descriptorpb.FieldDescriptorProto{
			scalarField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			scalarField("id", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		},
	}

	return &descriptorpb.FileDescriptorProto{
		Name:       proto.String("test/v1/contract.proto"),
		Package:    proto.String("test.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/api/annotations.proto", "google/protobuf/wrappers.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			request,
			payload,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("ContractService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					httpMethod("GetNested", "/v1/{name=tenants/*/users/*}"),
					httpMethod("GetSingle", "/v1/users/{id}"),
				},
			},
		},
	}
}

func scalarField(
	name string,
	number int32,
	fieldType descriptorpb.FieldDescriptorProto_Type,
) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   fieldType.Enum(),
	}
}

func messageField(name string, number int32, typeName string) *descriptorpb.FieldDescriptorProto {
	field := scalarField(name, number, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	field.TypeName = proto.String(typeName)
	return field
}

func httpMethod(name string, path string) *descriptorpb.MethodDescriptorProto {
	options := &descriptorpb.MethodOptions{}
	proto.SetExtension(options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{Get: path},
	})
	return &descriptorpb.MethodDescriptorProto{
		Name:       proto.String(name),
		InputType:  proto.String(".test.v1.GetRequest"),
		OutputType: proto.String(".test.v1.Payload"),
		Options:    options,
	}
}

func descriptorClosure(files ...protoreflect.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	seen := make(map[string]struct{})
	result := make([]*descriptorpb.FileDescriptorProto, 0, len(files))
	var visit func(protoreflect.FileDescriptor)
	visit = func(file protoreflect.FileDescriptor) {
		if _, ok := seen[file.Path()]; ok {
			return
		}
		seen[file.Path()] = struct{}{}
		imports := file.Imports()
		for i := 0; i < imports.Len(); i++ {
			visit(imports.Get(i).FileDescriptor)
		}
		result = append(result, protodesc.ToFileDescriptorProto(file))
	}
	for _, file := range files {
		visit(file)
	}
	return result
}

package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
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
		"int64Value?: string;",
		"uint64Value?: string;",
		"sint64Value?: string;",
		"fixed64Value?: string;",
		"sfixed64Value?: string;",
		"optionalInt64?: string;",
		"repeatedInt64?: string[];",
		"uint64ByKey?: { [key: string]: string };",
		"type wellKnownInt64Value = null | string;",
		"type wellKnownUInt64Value = null | string;",
	} {
		assert.Contains(t, output, declaration)
	}

	for _, declaration := range []string{
		"int32Value?: number;",
		"uint32Value?: number;",
		"floatValue?: number;",
		"doubleValue?: number;",
		"type wellKnownInt32Value = null | number;",
		"type wellKnownUInt32Value = null | number;",
	} {
		assert.Contains(t, output, declaration)
	}
	assert.NotContains(t, output, "@ts-nocheck")
	assert.NotContains(t, output, " | undefined;")
}

func TestGenerateEmitsTypeSafeHTTPRuntimeGuards(t *testing.T) {
	output := generateContractFixture(t)

	assert.Contains(t, output, "import type { ClientTransport, DuplexStream } from '@servora/proto-utils/transport';")
	assert.Contains(t, output, "return transport.unary<Payload>(")
	assert.Contains(t, output, "if (request.name === undefined || request.name === null || request.name === '') {")
	assert.Contains(t, output, "if (request.payload === undefined || request.payload === null) {")
	assert.Contains(t, output, "if (request.enabled !== undefined && request.enabled !== null) {")
	assert.Contains(t, output, "if (request.pageSize !== undefined && request.pageSize !== null) {")
	assert.NotContains(t, output, ") as Promise<")
}

func TestGenerateEncodesHTTPPathVariablesByTemplateShape(t *testing.T) {
	output := generateContractFixture(t)

	assert.Contains(t, output, "const path = `v1/${encodeMultiSegmentPath(request.name)}`;")
	assert.Contains(t, output, "const path = `v1/users/${encodePathSegment(request.id)}`;")
	assert.Contains(t, output, "import { encodeMultiSegmentPath, encodePathSegment } from '@servora/proto-utils/transport';")
	assert.NotContains(t, output, "function encodePathSegment(")
	assert.NotContains(t, output, "function encodeMultiSegmentPath(")
	assert.NotContains(t, output, "const path = `v1/${request.name}`;")
	assert.NotContains(t, output, "const path = `v1/users/${request.id}`;")
}

func TestGenerateEmitsStreamingTransportContract(t *testing.T) {
	output := generateContractFixture(t)

	assert.Contains(t, output, "SyncNamed(\n    request: StreamRequest,\n  ): DuplexStream<StreamRequest, Payload>;")
	assert.NotContains(t, output, "export interface ClientTransport")
	assert.NotContains(t, output, "export interface DuplexStream")
}

func TestGenerateOmitsTransportImportsWithoutServices(t *testing.T) {
	file := contractFixtureDescriptor()
	file.Service = nil
	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{file.GetName()},
		ProtoFile: descriptorClosure(
			annotations.File_google_api_annotations_proto,
			annotations.File_google_api_field_behavior_proto,
			wrapperspb.File_google_protobuf_wrappers_proto,
		),
	}
	request.ProtoFile = append(request.ProtoFile, file)

	response, err := Generate(request)
	require.NoError(t, err)
	require.Len(t, response.File, 1)
	output := response.File[0].GetContent()
	assert.NotContains(t, output, "@servora/proto-utils/transport")
}

func TestGenerateSkipsPureGRPCServicesWithoutHTTPRules(t *testing.T) {
	file := contractFixtureDescriptor()
	file.Service = []*descriptorpb.ServiceDescriptorProto{
		{
			Name: new("PureGrpcService"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{
					Name:       new("InternalOp"),
					InputType:  new(".test.v1.GetRequest"),
					OutputType: new(".google.protobuf.Int64Value"),
				},
			},
		},
	}
	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{file.GetName()},
		ProtoFile: descriptorClosure(
			annotations.File_google_api_annotations_proto,
			annotations.File_google_api_field_behavior_proto,
			wrapperspb.File_google_protobuf_wrappers_proto,
		),
	}
	request.ProtoFile = append(request.ProtoFile, file)

	response, err := Generate(request)
	require.NoError(t, err)
	require.Len(t, response.File, 1)
	output := response.File[0].GetContent()
	assert.NotContains(t, output, "@servora/proto-utils/transport")
	assert.NotContains(t, output, "PureGrpcService")
	assert.NotContains(t, output, "createApiClient")
}

func TestGeneratedBidiStreamBehavior(t *testing.T) {
	output := generateContractFixture(t)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("未安装 Node.js，跳过生成客户端行为测试")
	}

	script := output + `
type CapturedCall = {
  path: string;
  meta: { service: string; method: string };
  frames: unknown[];
  closeSendCount: number;
};

const calls: CapturedCall[] = [];
const transport: ClientTransport = {
  async unary<T>(): Promise<T> {
    throw new Error('本测试不调用普通请求');
  },
  serverStream() {
    throw new Error('本测试不调用服务端流');
  },
  duplexStream<TIn, TOut>(
    path: string,
    meta: { service: string; method: string },
    encode?: (data: TIn) => unknown,
  ): DuplexStream<TIn, TOut> {
    const call: CapturedCall = { path, meta, frames: [], closeSendCount: 0 };
    calls.push(call);
    return {
      send(data: TIn) {
        call.frames.push(encode ? encode(data) : data);
      },
      closeSend() {
        call.closeSendCount++;
      },
      onEvent() {
        return () => {};
      },
      onError() {},
      close() {},
    };
  },
};

function assertEqual(actual: unknown, expected: unknown, label: string): void {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(label + ': ' + JSON.stringify(actual));
  }
}

const client = createContractServiceClient(transport);
const named = client.SyncNamed({
  parent: 'tenants/acme team',
  payload: { int64Value: '9223372036854775807' },
  filter: 'enabled & ready',
  pageSize: 25,
});
named.send({ payload: { int64Value: '42' } });
named.closeSend();
assertEqual(calls[0], {
  path: 'v1/tenants/acme%20team/sync?filter=enabled%20%26%20ready&pageSize=25',
  meta: { service: 'ContractService', method: 'SyncNamed' },
  frames: [{ int64Value: '42' }],
  closeSendCount: 1,
}, '命名 body、URL、查询参数或半关闭转发错误');

const fullFrame = { parent: 'ignored', payload: { int64Value: '84' }, filter: 'ignored' };
const full = client.SyncAll();
full.send(fullFrame);
assertEqual(calls[1].frames, [fullFrame], '完整消息帧被错误映射');

const bodyOnly = client.SyncBodyOnly();
bodyOnly.send({ payload: { int64Value: '126' } });
assertEqual(calls[2].frames, [{ int64Value: '126' }], '无 URL 绑定的命名 body 映射错误');
`

	dir := t.TempDir()
	transportSrc, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "web", "packages", "proto-utils", "src", "transport.ts"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "package.json"),
		[]byte(`{"name":"@servora/proto-utils","type":"module","exports":{"./transport":"./transport.ts"}}`),
		0o600,
	))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transport.ts"), transportSrc, 0o600))
	file := filepath.Join(dir, "generated-stream.test.ts")
	require.NoError(t, os.WriteFile(file, []byte(script), 0o600))
	command := exec.Command(node, file)
	command.Dir = dir
	combined, err := command.CombinedOutput()
	require.NoErrorf(t, err, "生成客户端行为执行失败：%s", combined)
}

func generateContractFixture(t *testing.T) string {
	t.Helper()

	file := contractFixtureDescriptor()
	request := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{file.GetName()},
		ProtoFile: descriptorClosure(
			annotations.File_google_api_annotations_proto,
			annotations.File_google_api_field_behavior_proto,
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
		Name: new("Payload"),
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: new("_optional_int64")},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{
				Name:    new("Uint64ByKeyEntry"),
				Options: &descriptorpb.MessageOptions{MapEntry: new(true)},
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
				Name:           new("optional_int64"),
				Number:         proto.Int32(10),
				Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:           descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
				OneofIndex:     proto.Int32(0),
				Proto3Optional: new(true),
			},
			{
				Name:   new("repeated_int64"),
				Number: proto.Int32(11),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
			},
			{
				Name:     new("uint64_by_key"),
				Number:   proto.Int32(12),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: new(".test.v1.Payload.Uint64ByKeyEntry"),
			},
			messageField("int64_wrapper", 13, ".google.protobuf.Int64Value"),
			messageField("uint64_wrapper", 14, ".google.protobuf.UInt64Value"),
			messageField("int32_wrapper", 15, ".google.protobuf.Int32Value"),
			messageField("uint32_wrapper", 16, ".google.protobuf.UInt32Value"),
		},
	}

	request := &descriptorpb.DescriptorProto{
		Name: new("GetRequest"),
		Field: []*descriptorpb.FieldDescriptorProto{
			scalarField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			scalarField("id", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			scalarField("enabled", 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL),
			scalarField("page_size", 4, descriptorpb.FieldDescriptorProto_TYPE_INT32),
		},
	}
	createRequest := &descriptorpb.DescriptorProto{
		Name: new("CreateRequest"),
		Field: []*descriptorpb.FieldDescriptorProto{
			requiredField(messageField("payload", 1, ".test.v1.Payload")),
		},
	}

	streamRequest := &descriptorpb.DescriptorProto{
		Name: new("StreamRequest"),
		Field: []*descriptorpb.FieldDescriptorProto{
			scalarField("parent", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			messageField("payload", 2, ".test.v1.Payload"),
			scalarField("filter", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			scalarField("page_size", 4, descriptorpb.FieldDescriptorProto_TYPE_INT32),
		},
	}
	bodyOnlyStreamRequest := &descriptorpb.DescriptorProto{
		Name: new("BodyOnlyStreamRequest"),
		Field: []*descriptorpb.FieldDescriptorProto{
			messageField("payload", 1, ".test.v1.Payload"),
		},
	}
	return &descriptorpb.FileDescriptorProto{
		Name:       new("test/v1/contract.proto"),
		Package:    new("test.v1"),
		Syntax:     new("proto3"),
		Dependency: []string{"google/api/annotations.proto", "google/api/field_behavior.proto", "google/protobuf/wrappers.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			request,
			createRequest,
			streamRequest,
			bodyOnlyStreamRequest,
			payload,
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: new("ContractService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					httpMethod("GetNested", "/v1/{name=tenants/*/users/*}"),
					httpMethod("GetSingle", "/v1/users/{id}"),
					httpPostMethod("Create", "/v1/users", "payload", ".test.v1.CreateRequest"),
					bidiHTTPPostMethod("SyncNamed", "/v1/{parent=tenants/*}/sync", "payload", ".test.v1.StreamRequest"),
					bidiHTTPPostMethod("SyncAll", "/v1/sync-all", "*", ".test.v1.StreamRequest"),
					bidiHTTPPostMethod("SyncBodyOnly", "/v1/sync-body", "payload", ".test.v1.BodyOnlyStreamRequest"),
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
		Name:   new(name),
		Number: new(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   fieldType.Enum(),
	}
}

func messageField(name string, number int32, typeName string) *descriptorpb.FieldDescriptorProto {
	field := scalarField(name, number, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	field.TypeName = new(typeName)
	return field
}

func httpMethod(name string, path string) *descriptorpb.MethodDescriptorProto {
	options := &descriptorpb.MethodOptions{}
	proto.SetExtension(options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{Get: path},
	})
	return &descriptorpb.MethodDescriptorProto{
		Name:       new(name),
		InputType:  new(".test.v1.GetRequest"),
		OutputType: new(".test.v1.Payload"),
		Options:    options,
	}
}

func requiredField(field *descriptorpb.FieldDescriptorProto) *descriptorpb.FieldDescriptorProto {
	field.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(field.Options, annotations.E_FieldBehavior, []annotations.FieldBehavior{annotations.FieldBehavior_REQUIRED})
	return field
}

func httpPostMethod(name, path, body, input string) *descriptorpb.MethodDescriptorProto {
	options := &descriptorpb.MethodOptions{}
	proto.SetExtension(options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Post{Post: path},
		Body:    body,
	})
	return &descriptorpb.MethodDescriptorProto{
		Name:       new(name),
		InputType:  new(input),
		OutputType: new(".test.v1.Payload"),
		Options:    options,
	}
}

func bidiHTTPPostMethod(name, path, body, input string) *descriptorpb.MethodDescriptorProto {
	method := httpPostMethod(name, path, body, input)
	method.ClientStreaming = new(true)
	method.ServerStreaming = new(true)
	return method
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

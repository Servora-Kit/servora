package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	validate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	tlsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/security/tls/v1"
	"github.com/Servora-Kit/servora/cmd/internal/plugintest"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func fixtureField(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Number: proto.Int32(number), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: kind.Enum()}
	if typeName != "" {
		f.TypeName = proto.String(typeName)
	}
	return f
}

func confRule(f *descriptorpb.FieldDescriptorProto, defaultValue *string, required bool) *descriptorpb.FieldDescriptorProto {
	f.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(f.Options, confv1.E_Field, &confv1.FieldRule{Default: defaultValue, Required: required})
	return f
}

func optional(f *descriptorpb.FieldDescriptorProto, index int32) *descriptorpb.FieldDescriptorProto {
	f.Proto3Optional = proto.Bool(true)
	f.OneofIndex = proto.Int32(index)
	return f
}

func configurationFixture() *descriptorpb.FileDescriptorProto {
	childName := optional(confRule(fixtureField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""), nil, true), 0)
	childName.Options = proto.Clone(childName.Options).(*descriptorpb.FieldOptions)
	proto.SetExtension(childName.Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_String_{String_: &validate.StringRules{MinLen: proto.Uint64(1)}}})
	child := &descriptorpb.DescriptorProto{Name: proto.String("Child"), OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("_name")}}, Field: []*descriptorpb.FieldDescriptorProto{childName}}
	root := &descriptorpb.DescriptorProto{
		Name:    proto.String("ConfigRoot"),
		Options: &descriptorpb.MessageOptions{},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("selection")}, {Name: proto.String("_enabled")}, {Name: proto.String("_count")}, {Name: proto.String("_label")},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("EntriesEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)}, Field: []*descriptorpb.FieldDescriptorProto{
				fixtureField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""), fixtureField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Child"),
			}},
			{Name: proto.String("LabelsEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)}, Field: []*descriptorpb.FieldDescriptorProto{
				fixtureField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""), fixtureField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
			}},
		},
		Field: []*descriptorpb.FieldDescriptorProto{
			fixtureField("child", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Child"),
			{Name: proto.String("items"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.Child")},
			{Name: proto.String("entries"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.ConfigRoot.EntriesEntry")},
			{Name: proto.String("selected"), Number: proto.Int32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.Child"), OneofIndex: proto.Int32(0)},
			optional(confRule(fixtureField("enabled", 5, descriptorpb.FieldDescriptorProto_TYPE_BOOL, ""), nil, true), 1),
			optional(confRule(fixtureField("count", 6, descriptorpb.FieldDescriptorProto_TYPE_INT32, ""), proto.String("7"), false), 2),
			optional(confRule(fixtureField("label", 7, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""), proto.String(""), false), 3),
			confRule(fixtureField("span", 8, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".google.protobuf.Duration"), proto.String("5s"), false),
			{Name: proto.String("tags"), Number: proto.Int32(9), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), Options: &descriptorpb.FieldOptions{}},
			fixtureField("external", 10, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".servora.core.v1.Server.Listen"),
			{Name: proto.String("labels"), Number: proto.Int32(11), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.ConfigRoot.LabelsEntry"), Options: &descriptorpb.FieldOptions{}},
			fixtureField("external_tls", 12, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".servora.security.tls.v1.TLS"),
		},
	}
	business := &descriptorpb.DescriptorProto{Name: proto.String("BusinessRequest"), Field: []*descriptorpb.FieldDescriptorProto{
		fixtureField("child", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Child"),
	}}
	requiredParent := &descriptorpb.DescriptorProto{Name: proto.String("RequiredContainer"), Field: []*descriptorpb.FieldDescriptorProto{
		confRule(fixtureField("child", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Child"), nil, true),
	}}
	recursiveName := fixtureField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "")
	recursiveName.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(recursiveName.Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_String_{String_: &validate.StringRules{MinLen: proto.Uint64(2)}}})
	recursive := &descriptorpb.DescriptorProto{Name: proto.String("Recursive"), Options: &descriptorpb.MessageOptions{}, Field: []*descriptorpb.FieldDescriptorProto{
		recursiveName,
		fixtureField("child", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Recursive"),
		{Name: proto.String("children"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.Recursive")},
	}}
	proto.SetExtension(recursive.Options, confv1.E_Section, true)
	proto.SetExtension(root.Options, confv1.E_Section, true)
	proto.SetExtension(root.Field[8].Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_Repeated{Repeated: &validate.RepeatedRules{MinItems: proto.Uint64(1)}}})
	proto.SetExtension(root.Field[10].Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_Map{Map: &validate.MapRules{
		Keys:   &validate.FieldRules{Type: &validate.FieldRules_String_{String_: &validate.StringRules{MinLen: proto.Uint64(2)}}},
		Values: &validate.FieldRules{Type: &validate.FieldRules_String_{String_: &validate.StringRules{MinLen: proto.Uint64(1)}}},
	}}})
	return &descriptorpb.FileDescriptorProto{
		Name: proto.String("fixture.proto"), Package: proto.String("fixture"), Syntax: proto.String("proto3"),
		Options:     &descriptorpb.FileOptions{GoPackage: proto.String("example.com/fixture;fixture")},
		Dependency:  []string{"servora/conf/v1/annotations.proto", "servora/core/v1/bootstrap.proto", "servora/security/tls/v1/config.proto", "buf/validate/validate.proto", "google/protobuf/duration.proto"},
		MessageType: []*descriptorpb.DescriptorProto{child, root, business, requiredParent, recursive},
		Service:     []*descriptorpb.ServiceDescriptorProto{{Name: proto.String("BusinessService"), Method: []*descriptorpb.MethodDescriptorProto{{Name: proto.String("Execute"), InputType: proto.String(".fixture.BusinessRequest"), OutputType: proto.String(".fixture.BusinessRequest")}}}},
	}
}

func requestFor(f *descriptorpb.FileDescriptorProto) *pluginpb.CodeGeneratorRequest {
	return &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{f.GetName()}, Parameter: proto.String("paths=source_relative"),
		ProtoFile: append(plugintest.DescriptorClosure(confv1.File_servora_conf_v1_annotations_proto, corev1.File_servora_core_v1_bootstrap_proto, tlsv1.File_servora_security_tls_v1_config_proto, validate.File_buf_validate_validate_proto, durationpb.File_google_protobuf_duration_proto), f),
	}
}

func TestGeneratedConfigurationBehavior(t *testing.T) {
	request := requestFor(configurationFixture())
	plugin, err := (protogen.Options{}).New(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(plugin); err != nil {
		t.Fatal(err)
	}
	files := plugintest.ResponseFiles(plugin)

	// 使用标准 Go 生成器验证可选字段、map entry 和 oneof 的真实类型。
	command := exec.Command("go", "run", "google.golang.org/protobuf/cmd/protoc-gen-go")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "GOWORK=off")
	input, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	command.Stdin = strings.NewReader(string(input))
	output, err := command.Output()
	if err != nil {
		t.Fatalf("protoc-gen-go: %v", err)
	}
	var response pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(output, &response); err != nil {
		t.Fatal(err)
	}
	if response.GetError() != "" {
		t.Fatal(response.GetError())
	}
	for _, generated := range response.File {
		files[generated.GetName()] = generated.GetContent()
	}

	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = "module example.com/fixture\n\ngo 1.27.0\n\nrequire github.com/Servora-Kit/servora v0.0.0\nreplace github.com/Servora-Kit/servora => " + repoRoot + "\n"
	files["fixture_test.go"] = `package fixture
import (
 "strings"
 "testing"
 protovalidate "buf.build/go/protovalidate"
 "google.golang.org/protobuf/proto"
 corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
 tlsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/security/tls/v1"
 "google.golang.org/protobuf/types/known/durationpb"
)
func value[T any](v T) *T { return &v }
func base() *ConfigRoot { return &ConfigRoot{Enabled: value(false), Tags: []string{"present"}} }
func fails(t *testing.T, got error, path string) {
 t.Helper()
 if got == nil || !strings.Contains(got.Error(), path) { t.Fatalf("expected %q, got %v", path, got) }
}
func TestApplyContract(t *testing.T) {
 fails(t, (&ConfigRoot{Tags: []string{"present"}}).Apply(), "enabled")
 p := base()
 if err := p.Apply(); err != nil { t.Fatal(err) }
 if p.Child != nil || p.Count == nil || *p.Count != 7 || p.Label == nil || *p.Label != "" || p.Span.AsDuration() != durationpb.New(5e9).AsDuration() { t.Fatalf("defaults and field state: %+v", p) }
 snapshot := proto.Clone(p)
 if err := p.Apply(); err != nil || !proto.Equal(p, snapshot) { t.Fatalf("Apply not stable: %v, %+v", err, p) }
 p = base(); p.Count = value(int32(0)); p.Label = value("")
 if err := p.Apply(); err != nil || *p.Count != 0 || *p.Label != "" || !proto.Equal(p.Span, durationpb.New(5e9)) { t.Fatalf("explicit zero lost: %+v, %v", p, err) }
 p = base(); p.Child = &Child{}; fails(t, p.Apply(), "child: name")
 p.Child.Name = value(""); fails(t, p.Apply(), "child:")
 p = base(); p.Items = []*Child{nil}; fails(t, p.Apply(), "items[0]")
 p = base(); p.Items = []*Child{{Name: value("ok")}, {}}; fails(t, p.Apply(), "items[1]: name")
 p = base(); p.Entries = map[string]*Child{"z": {}, "a": {}}; fails(t, p.Apply(), "entries[\"a\"]: name")
 p = base(); p.Entries = map[string]*Child{"a": nil}; fails(t, p.Apply(), "entries[\"a\"]: nil")
 p = base(); p.Selection = &ConfigRoot_Selected{}; fails(t, p.Apply(), "selected: nil")
 p = base(); p.Selection = &ConfigRoot_Selected{Selected: &Child{}}; fails(t, p.Apply(), "selected: name")
 p = base(); p.External = &corev1.Server_Listen{}; fails(t, p.Apply(), "external: addr")
 p = base(); p.ExternalTls = &tlsv1.TLS{}; if err := p.Apply(); err != nil { t.Fatalf("unannotated imported TLS: %v", err) }
 r := &RequiredContainer{}; fails(t, r.Apply(), "child is required")
 r.Child = &Child{}; fails(t, r.Apply(), "child: name")
 r.Child.Name = value("valid"); if err := r.Apply(); err != nil { t.Fatal(err) }
 node := &Recursive{Name: "root", Child: &Recursive{Name: "x"}}
 if err := node.Apply(); err == nil || !strings.HasPrefix(err.Error(), "child:") { t.Fatalf("same-type child was not validated: %v", err) }
 node.Child.Name = "valid"
 node.Children = []*Recursive{{Name: "x"}}
 if err := node.Apply(); err == nil || !strings.HasPrefix(err.Error(), "children[0]:") { t.Fatalf("same-type collection item was not validated: %v", err) }
 node.Children[0].Name = "valid"
 if err := node.Apply(); err != nil { t.Fatalf("valid recursive config: %v", err) }
 b := &corev1.Bootstrap{Obs: &corev1.Observability{Log: &corev1.Log{Backends: []*corev1.Log_LogBackend{{Backend: &corev1.Log_LogBackend_File{File: &corev1.Log_FileBackend{}}}}}}}
 fails(t, b.Apply(), "obs: log: backends[0]: file: path")
 trace := &corev1.Bootstrap{Obs: &corev1.Observability{Trace: &corev1.Trace{SamplingRatio: value(1.1)}}}
 fails(t, trace.Apply(), "obs: trace:")
 p = base(); p.Tags = nil; fails(t, p.Apply(), "tags")
 if _, ok := any(&BusinessRequest{}).(interface{ Apply() error }); ok { t.Fatal("RPC request unexpectedly has configuration Apply") }
}
func TestMapRuleErrorOrdering(t *testing.T) {
 cases := []struct { name string; labels map[string]string; first string }{
  {"keys", map[string]string{"b": "ok", "a": "ok"}, "labels[\"a\"]"},
  {"values", map[string]string{"zz": "", "aa": ""}, "labels[\"aa\"]"},
 }
 for _, test := range cases {
  t.Run(test.name, func(t *testing.T) {
   for range 32 {
    p := base(); p.Labels = test.labels
    err := p.Apply()
    violation, ok := err.(*protovalidate.ValidationError)
    if !ok || len(violation.Violations) != 2 { t.Fatalf("expected two map violations, got %v", err) }
    first := protovalidate.FieldPathString(violation.Violations[0].Proto.GetField())
    if first != test.first { t.Fatalf("unstable first map violation: %s, want %s", first, test.first) }
   }
  })
 }
}
`
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command = exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated behavior: %v\n%s", err, output)
	}
}

func TestImportedValueRulesWithDirectorySplit(t *testing.T) {
	code := fixtureField("code", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "")
	code.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(code.Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_String_{String_: &validate.StringRules{MinLen: proto.Uint64(2)}}})
	label := fixtureField("label", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "")
	label.Options = &descriptorpb.FieldOptions{}
	proto.SetExtension(label.Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_String_{String_: &validate.StringRules{MinLen: proto.Uint64(2)}}})
	external := &descriptorpb.FileDescriptorProto{
		Name: proto.String("external/validated.proto"), Package: proto.String("fixture.external"), Syntax: proto.String("proto3"),
		Options:    &descriptorpb.FileOptions{GoPackage: proto.String("example.com/fixture/external;external")},
		Dependency: []string{"buf/validate/validate.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Leaf"), Field: []*descriptorpb.FieldDescriptorProto{label}},
			{Name: proto.String("Empty")},
			{Name: proto.String("Pure"), Field: []*descriptorpb.FieldDescriptorProto{{Name: proto.String("items"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.external.Empty")}}},
			{Name: proto.String("Child"), OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("choice")}},
				NestedType: []*descriptorpb.DescriptorProto{{Name: proto.String("ByKeyEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)}, Field: []*descriptorpb.FieldDescriptorProto{
					fixtureField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""), fixtureField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.external.Empty"),
				}}},
				Field: []*descriptorpb.FieldDescriptorProto{
					code,
					fixtureField("leaf", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.external.Leaf"),
					{Name: proto.String("items"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.external.Empty")},
					{Name: proto.String("by_key"), Number: proto.Int32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.external.Child.ByKeyEntry")},
					{Name: proto.String("selected"), Number: proto.Int32(5), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.external.Empty"), OneofIndex: proto.Int32(0)},
					{Name: proto.String("literal"), Number: proto.Int32(6), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), OneofIndex: proto.Int32(0)},
				}},
		},
	}
	parent := &descriptorpb.DescriptorProto{Name: proto.String("Parent"), Options: &descriptorpb.MessageOptions{},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("selection")}},
		NestedType: []*descriptorpb.DescriptorProto{{Name: proto.String("ByKeyEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)}, Field: []*descriptorpb.FieldDescriptorProto{
			fixtureField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""), fixtureField("value", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.external.Child"),
		}}},
		Field: []*descriptorpb.FieldDescriptorProto{
			fixtureField("foreign", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.external.Child"),
			{Name: proto.String("children"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.external.Child")},
			{Name: proto.String("by_key"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.Parent.ByKeyEntry")},
			{Name: proto.String("selected"), Number: proto.Int32(4), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".fixture.external.Child"), OneofIndex: proto.Int32(0)},
			fixtureField("foreign_pure", 5, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.external.Pure"),
		},
	}
	proto.SetExtension(parent.Options, confv1.E_Section, true)
	fixture := &descriptorpb.FileDescriptorProto{Name: proto.String("fixture.proto"), Package: proto.String("fixture"), Syntax: proto.String("proto3"),
		Options:     &descriptorpb.FileOptions{GoPackage: proto.String("example.com/fixture;fixture")},
		Dependency:  []string{"servora/conf/v1/annotations.proto", "external/validated.proto"},
		MessageType: []*descriptorpb.DescriptorProto{parent},
	}
	dependency := plugintest.DescriptorClosure(confv1.File_servora_conf_v1_annotations_proto, validate.File_buf_validate_validate_proto)
	externalRequest := &pluginpb.CodeGeneratorRequest{FileToGenerate: []string{external.GetName()}, Parameter: proto.String("paths=source_relative"), ProtoFile: append(append([]*descriptorpb.FileDescriptorProto{}, dependency...), external)}
	externalPlugin, err := (protogen.Options{}).New(externalRequest)
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(externalPlugin); err != nil {
		t.Fatal(err)
	}
	if files := plugintest.ResponseFiles(externalPlugin); len(files) != 0 {
		t.Fatalf("value-only foreign file unexpectedly generated conf API: %v", plugintest.SortedKeys(files))
	}
	parentRequest := &pluginpb.CodeGeneratorRequest{FileToGenerate: []string{fixture.GetName()}, Parameter: proto.String("paths=source_relative"), ProtoFile: append(append(append([]*descriptorpb.FileDescriptorProto{}, dependency...), external), fixture)}
	parentPlugin, err := (protogen.Options{}).New(parentRequest)
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(parentPlugin); err != nil {
		t.Fatal(err)
	}
	files := plugintest.ResponseFiles(parentPlugin)
	standardRequest := proto.Clone(parentRequest).(*pluginpb.CodeGeneratorRequest)
	standardRequest.FileToGenerate = []string{external.GetName(), fixture.GetName()}
	input, err := proto.Marshal(standardRequest)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "run", "google.golang.org/protobuf/cmd/protoc-gen-go")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "GOWORK=off")
	command.Stdin = strings.NewReader(string(input))
	output, err := command.Output()
	if err != nil {
		t.Fatalf("protoc-gen-go: %v", err)
	}
	var response pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(output, &response); err != nil {
		t.Fatal(err)
	}
	if response.GetError() != "" {
		t.Fatal(response.GetError())
	}
	for _, generated := range response.File {
		files[generated.GetName()] = generated.GetContent()
	}
	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files["go.mod"] = "module example.com/fixture\n\ngo 1.27.0\n\nrequire github.com/Servora-Kit/servora v0.0.0\nreplace github.com/Servora-Kit/servora => " + repoRoot + "\n"
	files["fixture_test.go"] = `package fixture
import (
 "errors"
 protovalidate "buf.build/go/protovalidate"
 "strings"
 "testing"
 ext "example.com/fixture/external"
)
func expect(t *testing.T, err error, path string) {
 t.Helper()
 if err == nil || !strings.Contains(err.Error(), path) { t.Fatalf("expected %s, got %v", path, err) }
}
func expectValidation(t *testing.T, err error, prefix, field string) {
 t.Helper()
 if err == nil || !strings.HasPrefix(err.Error(), prefix) { t.Fatalf("expected prefix %q, got %v", prefix, err) }
 var validationErr *protovalidate.ValidationError
 if !errors.As(err, &validationErr) { t.Fatalf("validation error type lost: %T", err) }
 for _, violation := range validationErr.Violations {
  if protovalidate.FieldPathString(violation.Proto.GetField()) == field { return }
 }
 t.Fatalf("expected rule violation at %s, got %v", field, validationErr)
}
func TestExternalValueRules(t *testing.T) {
 if err := (&Parent{}).Apply(); err != nil { t.Fatal(err) }
 expectValidation(t, (&Parent{Foreign: &ext.Child{Code: "x"}}).Apply(), "foreign:", "code")
 expectValidation(t, (&Parent{Foreign: &ext.Child{Code: "okay", Leaf: &ext.Leaf{Label: "x"}}}).Apply(), "foreign:", "leaf.label")
 expect(t, (&Parent{Foreign: &ext.Child{Code: "ok", Items: []*ext.Empty{nil}}}).Apply(), "foreign: items[0]: nil")
 expect(t, (&Parent{Foreign: &ext.Child{Code: "ok", ByKey: map[string]*ext.Empty{"z": nil, "a": nil}}}).Apply(), "foreign: by_key[\"a\"]: nil")
 expect(t, (&Parent{ForeignPure: &ext.Pure{Items: []*ext.Empty{nil}}}).Apply(), "foreign_pure: items[0]: nil")
 var selected *ext.Child_Selected
 expect(t, (&Parent{Foreign: &ext.Child{Code: "ok", Choice: selected}}).Apply(), "foreign: selected: nil oneof branch")
 var parentSelected *Parent_Selected
 expect(t, (&Parent{Selection: parentSelected}).Apply(), "selected: nil oneof branch")
 expect(t, (&Parent{Children: []*ext.Child{nil}}).Apply(), "children[0]: nil")
 expectValidation(t, (&Parent{Children: []*ext.Child{{Code: "x"}}}).Apply(), "children[0]:", "code")
 expectValidation(t, (&Parent{ByKey: map[string]*ext.Child{"z": {Code: "x"}, "a": {Code: "x"}}}).Apply(), "by_key[\"a\"]:", "code")
 expectValidation(t, (&Parent{Selection: &Parent_Selected{Selected: &ext.Child{Code: "x"}}}).Apply(), "selected:", "code")
 if err := (&Parent{Foreign: &ext.Child{Code: "okay"}}).Apply(); err != nil { t.Fatal(err) }
}
`
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	command = exec.Command("go", "test", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("separate-directory generated behavior: %v\n%s", err, output)
	}
}

func TestInvalidConfigurationDeclarations(t *testing.T) {
	cases := []struct {
		name   string
		change func(*descriptorpb.FileDescriptorProto)
		want   string
	}{
		{"conflicting empty default", func(f *descriptorpb.FileDescriptorProto) {
			field := f.MessageType[1].Field[6]
			confRule(field, proto.String(""), true)
		}, "fixture.ConfigRoot.label"},
		{"scalar missing field state", func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[1].Field = append(f.MessageType[1].Field, confRule(fixtureField("plain", 13, descriptorpb.FieldDescriptorProto_TYPE_INT32, ""), proto.String("1"), false))
		}, "fixture.ConfigRoot.plain"},
		{"repeated required", func(f *descriptorpb.FileDescriptorProto) { confRule(f.MessageType[1].Field[8], nil, true) }, "fixture.ConfigRoot.tags"},
		{"invalid duration", func(f *descriptorpb.FileDescriptorProto) {
			confRule(f.MessageType[1].Field[7], proto.String("not-a-duration"), false)
		}, "fixture.ConfigRoot.span"},
		{"invalid bool literal", func(f *descriptorpb.FileDescriptorProto) {
			confRule(f.MessageType[1].Field[4], proto.String("perhaps"), false)
		}, "fixture.ConfigRoot.enabled"},
		{"bytes default unsupported", func(f *descriptorpb.FileDescriptorProto) {
			root := f.MessageType[1]
			root.OneofDecl = append(root.OneofDecl, &descriptorpb.OneofDescriptorProto{Name: proto.String("_binary")})
			root.Field = append(root.Field, optional(confRule(fixtureField("binary", 13, descriptorpb.FieldDescriptorProto_TYPE_BYTES, ""), proto.String("x"), false), 4))
		}, "fixture.ConfigRoot.binary"},
		{"mutually required branches", func(f *descriptorpb.FileDescriptorProto) {
			root := f.MessageType[1]
			confRule(root.Field[3], nil, true)
			other := confRule(&descriptorpb.FieldDescriptorProto{Name: proto.String("other"), Number: proto.Int32(13), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), OneofIndex: proto.Int32(0)}, nil, true)
			root.Field = append(root.Field[:4], append([]*descriptorpb.FieldDescriptorProto{other}, root.Field[4:]...)...)
		}, "fixture.ConfigRoot.selected"},
		{"oneof default", func(f *descriptorpb.FileDescriptorProto) {
			confRule(f.MessageType[1].Field[3], proto.String("foo"), false)
		}, "fixture.ConfigRoot.selected"},
		{"validation ignore", func(f *descriptorpb.FileDescriptorProto) {
			field := f.MessageType[1].Field[8]
			proto.SetExtension(field.Options, validate.E_Field, &validate.FieldRules{Ignore: validate.Ignore_IGNORE_ALWAYS.Enum()})
		}, "fixture.ConfigRoot.tags"},
		{"nested validation ignore", func(f *descriptorpb.FileDescriptorProto) {
			field := f.MessageType[1].Field[8]
			proto.SetExtension(field.Options, validate.E_Field, &validate.FieldRules{Type: &validate.FieldRules_Repeated{Repeated: &validate.RepeatedRules{Items: &validate.FieldRules{Ignore: validate.Ignore_IGNORE_ALWAYS.Enum()}}}})
		}, "fixture.ConfigRoot.tags"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := configurationFixture()
			tc.change(fixture)
			plugin, err := (protogen.Options{}).New(requestFor(fixture))
			if err != nil {
				t.Fatal(err)
			}
			err = generate(plugin)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want field path %s, got %v", tc.want, err)
			}
		})
	}
}

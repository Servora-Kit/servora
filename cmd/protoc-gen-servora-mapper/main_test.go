package main

import (
	"sort"
	"strings"
	"testing"

	mapperpb "github.com/Servora-Kit/servora/api/gen/go/servora/mapper/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type mapperFieldSpec struct {
	name   string
	number int32
	rename string
}

type mapperMessageSpec struct {
	name       string
	rule       *mapperpb.MapperRule
	fields     []mapperFieldSpec
	plainField bool
}

type mapperFileSpec struct {
	name     string
	pkg      string
	goPkg    string
	generate bool
	messages []mapperMessageSpec
}

func collectMapperDeps(fd protoreflect.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	seen := map[string]bool{}
	var out []*descriptorpb.FileDescriptorProto

	var visit func(f protoreflect.FileDescriptor)
	visit = func(f protoreflect.FileDescriptor) {
		if seen[f.Path()] {
			return
		}
		seen[f.Path()] = true
		imports := f.Imports()
		for i := 0; i < imports.Len(); i++ {
			visit(imports.Get(i).FileDescriptor)
		}
		out = append(out, protodesc.ToFileDescriptorProto(f))
	}
	visit(fd)
	return out
}

func runMapperPluginScenario(t *testing.T, files []mapperFileSpec) (*protogen.Plugin, error) {
	t.Helper()

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: collectMapperDeps(mapperpb.File_servora_mapper_v1_mapper_proto),
	}
	for _, fs := range files {
		fp := buildMapperFileDescriptorProto(t, fs)
		req.ProtoFile = append(req.ProtoFile, fp)
		if fs.generate {
			req.FileToGenerate = append(req.FileToGenerate, fs.name)
		}
	}

	gen, err := protogen.Options{}.New(req)
	if err != nil {
		t.Fatalf("protogen.Options.New: %v", err)
	}
	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		if err := processFile(gen, f); err != nil {
			return gen, err
		}
	}
	return gen, nil
}

func buildMapperFileDescriptorProto(t *testing.T, fs mapperFileSpec) *descriptorpb.FileDescriptorProto {
	t.Helper()

	fp := &descriptorpb.FileDescriptorProto{
		Name:       proto.String(fs.name),
		Package:    proto.String(fs.pkg),
		Syntax:     proto.String(protoreflect.Proto3.String()),
		Dependency: []string{"google/protobuf/descriptor.proto", "servora/mapper/v1/mapper.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(fs.goPkg),
		},
	}

	for _, msg := range fs.messages {
		mp := &descriptorpb.DescriptorProto{Name: proto.String(msg.name)}
		if msg.rule != nil {
			opts := &descriptorpb.MessageOptions{}
			proto.SetExtension(opts, mapperpb.E_Mapper, msg.rule)
			mp.Options = opts
		}
		for _, field := range msg.fields {
			fp := &descriptorpb.FieldDescriptorProto{
				Name:     proto.String(field.name),
				Number:   proto.Int32(field.number),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				JsonName: proto.String(toJSONName(field.name)),
			}
			if field.rename != "" {
				opts := &descriptorpb.FieldOptions{}
				proto.SetExtension(opts, mapperpb.E_MapperField, &mapperpb.MapperFieldRule{Rename: field.rename})
				fp.Options = opts
			}
			mp.Field = append(mp.Field, fp)
		}
		if msg.plainField {
			mp.Field = append(mp.Field, &descriptorpb.FieldDescriptorProto{
				Name:     proto.String("name"),
				Number:   proto.Int32(100),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				JsonName: proto.String("name"),
			})
		}
		fp.MessageType = append(fp.MessageType, mp)
	}
	return fp
}

func generatedMapperFiles(t *testing.T, gen *protogen.Plugin) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range gen.Response().File {
		out[f.GetName()] = f.GetContent()
	}
	return out
}

func lookupMapperFile(t *testing.T, files map[string]string) string {
	t.Helper()
	var matches []string
	for k := range files {
		if strings.HasSuffix(k, "_mapper.gen.go") {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("expected *_mapper.gen.go in output, got: %v", keysOf(files))
	}
	if len(matches) > 1 {
		t.Fatalf("expected exactly one mapper output, got: %v", matches)
	}
	return files[matches[0]]
}

func keysOf[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func TestNoMapperAnnotations_NoFileGenerated(t *testing.T) {
	gen, err := runMapperPluginScenario(t, []mapperFileSpec{
		{
			name:     "example/v1/plain.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			messages: []mapperMessageSpec{{name: "Plain", plainField: true}},
		},
	})
	if err != nil {
		t.Fatalf("processFile returned error: %v", err)
	}

	files := generatedMapperFiles(t, gen)
	if len(files) != 0 {
		t.Fatalf("expected no output, got: %v", keysOf(files))
	}
}

func TestMapperAnnotations_GenerateConfig(t *testing.T) {
	gen, err := runMapperPluginScenario(t, []mapperFileSpec{
		{
			name:     "example/v1/application.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			messages: []mapperMessageSpec{
				{
					name: "Application",
					rule: &mapperpb.MapperRule{IgnoreRead: []string{
						"password_hash",
						"internal_note",
					}},
					fields: []mapperFieldSpec{
						{name: "id", number: 1, rename: "ID"},
						{name: "client_id", number: 2, rename: "ClientID"},
					},
					plainField: true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("processFile returned error: %v", err)
	}

	content := lookupMapperFile(t, generatedMapperFiles(t, gen))
	assertContains(t, content, "func ApplicationMapper() *mapper.Config")
	assertContains(t, content, "IgnoreRead: []string{\"password_hash\", \"internal_note\"}")
	assertContains(t, content, "\"ClientID\": \"ClientId\"")
	assertContains(t, content, "\"ID\":")
	assertContains(t, content, "\"Id\"")
	assertNotContains(t, content, "MapperPlan")
	assertNotContains(t, content, "ConverterKind")

	if strings.Index(content, "\"ClientID\":") > strings.Index(content, "\"ID\":") {
		t.Fatalf("field mapping output is not sorted:\n%s", content)
	}
}

func TestFieldAnnotationOnly_GeneratesConfig(t *testing.T) {
	gen, err := runMapperPluginScenario(t, []mapperFileSpec{
		{
			name:     "example/v1/user.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			messages: []mapperMessageSpec{
				{
					name: "User",
					fields: []mapperFieldSpec{
						{name: "client_id", number: 1, rename: "ClientID"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("processFile returned error: %v", err)
	}

	content := lookupMapperFile(t, generatedMapperFiles(t, gen))
	assertContains(t, content, "func UserMapper() *mapper.Config")
	assertContains(t, content, "\"ClientID\": \"ClientId\"")
}

func assertContains(t *testing.T, s, want string) {
	t.Helper()
	if !strings.Contains(s, want) {
		t.Fatalf("expected generated content to contain %q:\n%s", want, s)
	}
}

func assertNotContains(t *testing.T, s, want string) {
	t.Helper()
	if strings.Contains(s, want) {
		t.Fatalf("expected generated content not to contain %q:\n%s", want, s)
	}
}

func toJSONName(name string) string {
	parts := strings.Split(name, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

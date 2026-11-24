package plugintest

import (
	"reflect"
	"testing"

	confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestDescriptorClosureIsTopologicalAndUnique(t *testing.T) {
	files := DescriptorClosure(confv1.File_servora_conf_v1_annotations_proto)
	seen := make(map[string]int)
	for index, file := range files {
		if previous, duplicate := seen[file.GetName()]; duplicate {
			t.Fatalf("descriptor %q appears at %d and %d", file.GetName(), previous, index)
		}
		seen[file.GetName()] = index
		for _, dependency := range file.GetDependency() {
			dependencyIndex, present := seen[dependency]
			if !present || dependencyIndex >= index {
				t.Fatalf("dependency %q of %q is not before it", dependency, file.GetName())
			}
		}
	}
	if _, present := seen["servora/conf/v1/annotations.proto"]; !present {
		t.Fatal("conf annotations descriptor missing")
	}
}

func TestResponseFilesAndOnlyGeneratedFile(t *testing.T) {
	plugin, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		ProtoFile:      DescriptorClosure(confv1.File_servora_conf_v1_annotations_proto),
		FileToGenerate: []string{"servora/conf/v1/annotations.proto"},
		Parameter:      new("paths=source_relative"),
	})
	if err != nil {
		t.Fatalf("protogen.Options.New: %v", err)
	}
	first := plugin.NewGeneratedFile("example/v1/rules.gen.go", "example.com/example/v1")
	first.P("package test")
	second := plugin.NewGeneratedFile("example/v1/other.gen.go", "example.com/example/v1")
	second.P("package test")

	files := ResponseFiles(plugin)
	want := map[string]string{
		"example/v1/rules.gen.go": "package test\n",
		"example/v1/other.gen.go": "package test\n",
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
	if got := OnlyGeneratedFile(t, files, "rules.gen.go"); got != "package test\n" {
		t.Fatalf("OnlyGeneratedFile = %q", got)
	}
}

func TestAssertGeneratedGoCompiles(t *testing.T) {
	AssertGeneratedGoCompiles(t, `package fixture

import confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"

var _ = confv1.E_Section
`, "fixture")
}

func TestFindGeneratedFileRejectsMissingAndDuplicate(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{name: "missing", files: map[string]string{"a.go": "a"}, want: "no generated file"},
		{name: "duplicate", files: map[string]string{"a/rules.go": "a", "b/rules.go": "b"}, want: "multiple generated files"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := findGeneratedFile(test.files, "rules.go")
			if err == nil || !contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}

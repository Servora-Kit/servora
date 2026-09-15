// Package plugintest contains reusable protoc plugin test helpers.
package plugintest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DescriptorClosure returns roots and their imports in dependency-first order.
func DescriptorClosure(roots ...protoreflect.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	seen := make(map[string]struct{})
	var descriptors []*descriptorpb.FileDescriptorProto
	var visit func(protoreflect.FileDescriptor)
	visit = func(file protoreflect.FileDescriptor) {
		if file == nil {
			return
		}
		if _, duplicate := seen[file.Path()]; duplicate {
			return
		}
		seen[file.Path()] = struct{}{}
		imports := file.Imports()
		for index := range imports.Len() {
			visit(imports.Get(index).FileDescriptor)
		}
		descriptors = append(descriptors, protodesc.ToFileDescriptorProto(file))
	}
	for _, root := range roots {
		visit(root)
	}
	return descriptors
}

// ResponseFiles extracts generated path-to-content pairs from a plugin response.
func ResponseFiles(plugin *protogen.Plugin) map[string]string {
	files := make(map[string]string)
	if plugin == nil {
		return files
	}
	for _, file := range plugin.Response().File {
		files[file.GetName()] = file.GetContent()
	}
	return files
}

// OnlyGeneratedFile returns the content of the unique generated file matching baseName.
func OnlyGeneratedFile(t testing.TB, files map[string]string, baseName string) string {
	t.Helper()
	content, err := findGeneratedFile(files, baseName)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// AssertGeneratedGoCompiles type-checks generated source against the local Servora module.
func AssertGeneratedGoCompiles(t testing.TB, source, packageName string) {
	t.Helper()
	dir := t.TempDir()
	_, helperFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve plugintest helper path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(helperFile), "..", "..", ".."))
	goMod := "module sandbox\n\ngo 1.22\n\nrequire github.com/Servora-Kit/servora v0.0.0\nreplace github.com/Servora-Kit/servora => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	rewritten := strings.Replace(source, "package "+packageName, "package sandbox", 1)
	if rewritten == source {
		t.Fatalf("generated source does not declare package %q\n--- source ---\n%s", packageName, source)
	}
	if err := os.WriteFile(filepath.Join(dir, "generated.go"), []byte(rewritten), 0o644); err != nil {
		t.Fatalf("write generated source: %v", err)
	}
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet generated source: %v\n%s\n--- source ---\n%s", err, output, rewritten)
	}
}

func findGeneratedFile(files map[string]string, baseName string) (string, error) {
	matches := make([]string, 0, 1)
	for name := range files {
		if name == baseName || strings.HasSuffix(name, "/"+baseName) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no generated file %q; files: %v", baseName, SortedKeys(files))
	case 1:
		return files[matches[0]], nil
	default:
		return "", fmt.Errorf("multiple generated files %q: %v", baseName, matches)
	}
}

// SortedKeys returns map keys in lexical order for stable test diagnostics.
func SortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

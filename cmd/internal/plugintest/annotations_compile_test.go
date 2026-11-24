package plugintest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/wellknownimports"
)

func TestConfAnnotationsCompile(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve plugintest test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", ".."))
	sources := make(map[string]string)
	for importPath, filePath := range map[string]string{
		"servora/conf/v1/annotations.proto": filepath.Join(root, "api", "protos", "servora", "conf", "v1", "annotations.proto"),
		"test/annotations/compile.proto":     filepath.Join(filepath.Dir(testFile), "testdata", "annotations", "compile.proto"),
	} {
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read %s: %v", importPath, err)
		}
		sources[importPath] = string(content)
	}
	compiler := protocompile.Compiler{
		Resolver: wellknownimports.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(sources),
		}),
	}
	if _, err := compiler.Compile(t.Context(), "test/annotations/compile.proto"); err != nil {
		t.Fatalf("compile Conf annotations fixture: %v", err)
	}
}

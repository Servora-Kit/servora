package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/servora/authn/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// methodSpec describes a single RPC entry to materialize on a fake service.
type methodSpec struct {
	name string
	rule *authnpb.AuthnRule // nil → no method-level option
}

// serviceSpec describes a single service in the fake proto file.
type serviceSpec struct {
	name           string
	serviceDefault *authnpb.AuthnRule // nil → no service-level default
	methods        []methodSpec
}

// fileSpec describes a single proto file to feed the plugin.
type fileSpec struct {
	name     string // path used in CodeGeneratorRequest, e.g. "example/v1/greeting.proto"
	pkg      string // proto package, e.g. "example.v1"
	goPkg    string // full go_package option, e.g. "example.com/gen/example/v1;examplev1"
	generate bool   // whether to mark this file in FileToGenerate
	services []serviceSpec
}

// runPluginScenario constructs a fake protogen.Plugin from the given files,
// invokes generate(), and returns the generated file map (path → content).
//
// fatal == true means generate() returned a non-nil error (validation failure).
func runPluginScenario(t *testing.T, files []fileSpec) (gen *protogen.Plugin, err error) {
	t.Helper()

	descProto := protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto)
	authnProto := protodesc.ToFileDescriptorProto(authnpb.File_servora_authn_v1_annotations_proto)

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{descProto, authnProto},
	}

	for _, fs := range files {
		fp := buildFileDescriptorProto(t, fs)
		req.ProtoFile = append(req.ProtoFile, fp)
		if fs.generate {
			req.FileToGenerate = append(req.FileToGenerate, fs.name)
		}
	}

	gen, err = protogen.Options{}.New(req)
	if err != nil {
		t.Fatalf("protogen.Options.New: %v", err)
	}

	err = generate(gen)
	return gen, err
}

func buildFileDescriptorProto(t *testing.T, fs fileSpec) *descriptorpb.FileDescriptorProto {
	t.Helper()

	fp := &descriptorpb.FileDescriptorProto{
		Name:       proto.String(fs.name),
		Package:    proto.String(fs.pkg),
		Syntax:     proto.String(protoreflect.Proto3.String()),
		Dependency: []string{"google/protobuf/descriptor.proto", "servora/authn/v1/annotations.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(fs.goPkg),
		},
	}

	for _, svc := range fs.services {
		sp := &descriptorpb.ServiceDescriptorProto{
			Name: proto.String(svc.name),
		}
		if svc.serviceDefault != nil {
			opts := &descriptorpb.ServiceOptions{}
			proto.SetExtension(opts, authnpb.E_ServiceDefault, svc.serviceDefault)
			sp.Options = opts
		}
		for _, m := range svc.methods {
			mp := &descriptorpb.MethodDescriptorProto{
				Name:       proto.String(m.name),
				InputType:  proto.String(".google.protobuf.Empty"),
				OutputType: proto.String(".google.protobuf.Empty"),
			}
			if m.rule != nil {
				opts := &descriptorpb.MethodOptions{}
				proto.SetExtension(opts, authnpb.E_Rule, m.rule)
				mp.Options = opts
			}
			sp.Method = append(sp.Method, mp)
		}
		fp.Service = append(fp.Service, sp)
	}

	// Inject Empty placeholder so InputType/OutputType refs resolve. We
	// declare a local message named Empty in the package to avoid pulling
	// in google/protobuf/empty.proto as another dependency.
	emptyMsg := &descriptorpb.DescriptorProto{Name: proto.String("Empty")}
	fp.MessageType = append(fp.MessageType, emptyMsg)
	for _, svc := range fp.Service {
		for _, m := range svc.Method {
			m.InputType = proto.String("." + fs.pkg + ".Empty")
			m.OutputType = proto.String("." + fs.pkg + ".Empty")
		}
	}

	return fp
}

// generatedFiles extracts {path: content} from a Plugin's response.
func generatedFiles(t *testing.T, gen *protogen.Plugin) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range gen.Response().File {
		out[f.GetName()] = f.GetContent()
	}
	return out
}

func TestNoAnnotations_NoFileGenerated(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/empty.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "EmptyService",
					methods: []methodSpec{
						{name: "Noop"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	if len(files) != 0 {
		t.Fatalf("expected no generated files, got: %v", files)
	}
}

func TestMethodLevelPublic_GoesToPublicMethods(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					methods: []methodSpec{
						{name: "SayHello", rule: &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_PUBLIC}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	content := lookupAuthnFile(t, files)
	wantOp := `"/example.v1.GreetingService/SayHello"`
	if !strings.Contains(content, wantOp) {
		t.Fatalf("public methods missing %s\n--- generated ---\n%s", wantOp, content)
	}
}

func TestServiceDefault_MethodInherits(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authnpb.AuthnRule{
						Mode:    authnpb.AuthnRule_MODE_REQUIRED,
						Schemes: []string{"jwt"},
					},
					methods: []methodSpec{
						{name: "SayHello"}, // no method-level rule → inherits
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	content := lookupAuthnFile(t, files)
	wantKey := `"/example.v1.GreetingService/SayHello"`
	if !strings.Contains(content, wantKey) {
		t.Fatalf("inherited rule missing operation key %s\n--- generated ---\n%s", wantKey, content)
	}
	if !strings.Contains(content, `"jwt"`) {
		t.Fatalf("inherited rule missing schemes literal %q\n--- generated ---\n%s", "jwt", content)
	}
}

func TestMethodOverridesServiceDefault_PublicWins(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authnpb.AuthnRule{
						Mode:    authnpb.AuthnRule_MODE_REQUIRED,
						Schemes: []string{"jwt"},
					},
					methods: []methodSpec{
						{name: "PublicHello", rule: &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_PUBLIC}},
						{name: "PrivateHello"}, // inherits REQUIRED + jwt
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	content := lookupAuthnFile(t, files)
	publicOp := `"/example.v1.GreetingService/PublicHello"`
	privateOp := `"/example.v1.GreetingService/PrivateHello"`

	// PublicHello must appear in _publicMethods slice section.
	publicSection, schemesSection := splitSections(t, content)

	if !strings.Contains(publicSection, publicOp) {
		t.Errorf("PublicHello missing from _publicMethods section:\n%s", publicSection)
	}
	if strings.Contains(schemesSection, publicOp) {
		t.Errorf("PublicHello should NOT appear in _methodSchemes (was overridden):\n%s", schemesSection)
	}
	if !strings.Contains(schemesSection, privateOp) {
		t.Errorf("PrivateHello missing from _methodSchemes (should inherit jwt):\n%s", schemesSection)
	}
}

func TestInvalid_PublicWithSchemes(t *testing.T) {
	_, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/bad.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "BadService",
					methods: []methodSpec{
						{name: "BadOp", rule: &authnpb.AuthnRule{
							Mode:    authnpb.AuthnRule_MODE_PUBLIC,
							Schemes: []string{"jwt"},
						}},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for PUBLIC + non-empty schemes, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"example/v1/bad.proto", "BadService", "BadOp", "MODE_PUBLIC", "schemes"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q\n--- got ---\n%s", want, msg)
		}
	}
}

func TestInvalid_RequiredWithEmptySchemes(t *testing.T) {
	_, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/bad.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "BadService",
					methods: []methodSpec{
						{name: "BadOp", rule: &authnpb.AuthnRule{
							Mode: authnpb.AuthnRule_MODE_REQUIRED,
						}},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for REQUIRED + empty schemes, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"example/v1/bad.proto", "BadService", "BadOp", "MODE_REQUIRED", "schemes"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q\n--- got ---\n%s", want, msg)
		}
	}
}

func TestInvalid_UnspecifiedWithSchemes(t *testing.T) {
	_, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/bad.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "BadService",
					methods: []methodSpec{
						{name: "BadOp", rule: &authnpb.AuthnRule{
							Schemes: []string{"jwt"},
						}},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for UNSPECIFIED + non-empty schemes, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"example/v1/bad.proto", "BadService", "BadOp", "MODE_UNSPECIFIED", "schemes"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q\n--- got ---\n%s", want, msg)
		}
	}
}

// TestGeneratedAccessorsReturnIndependentCopies validates the produced Go file
// declares PublicMethods / MethodSchemes accessors that explicitly copy the
// backing slice and map (deep copy for slices inside the map). We assert on
// generated source presence rather than execute it (which would require a
// separate `go test` pipeline on the produced file).
func TestGeneratedAccessorsReturnIndependentCopies(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authnpb.AuthnRule{
						Mode:    authnpb.AuthnRule_MODE_REQUIRED,
						Schemes: []string{"jwt"},
					},
					methods: []methodSpec{
						{name: "SayHello"},
						{name: "Healthz", rule: &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_PUBLIC}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	content := lookupAuthnFile(t, files)
	// PublicMethods accessor must allocate a fresh slice and copy.
	for _, sig := range []string{
		"func PublicMethods() []string",
		"copy(",       // slice copy in PublicMethods
		"make([]string,", // fresh slice allocation
		"func MethodSchemes() map[string][]string",
	} {
		if !strings.Contains(content, sig) {
			t.Errorf("generated file missing %q\n--- generated ---\n%s", sig, content)
		}
	}
	// MethodSchemes must deep-copy slices (one copy per slice value).
	// We look for the loop that allocates a per-key slice copy.
	if !strings.Contains(content, "make(map[string][]string,") {
		t.Errorf("MethodSchemes() should allocate a fresh map")
	}
}

// TestGeneratedFileCompiles writes the produced authn_rules.gen.go into a
// throw-away module and runs `go vet` to verify it parses & type-checks. This
// guards against subtle template bugs (missing imports, malformed literals)
// that pure string-matching tests would miss.
func TestGeneratedFileCompiles(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authnpb.AuthnRule{
						Mode:    authnpb.AuthnRule_MODE_REQUIRED,
						Schemes: []string{"jwt"},
					},
					methods: []methodSpec{
						{name: "SayHello"},
						{name: "Healthz", rule: &authnpb.AuthnRule{Mode: authnpb.AuthnRule_MODE_PUBLIC}},
						{name: "AdminPurge", rule: &authnpb.AuthnRule{
							Mode:    authnpb.AuthnRule_MODE_REQUIRED,
							Schemes: []string{"mtls", "jwt"},
						}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	src := lookupAuthnFile(t, files)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sandbox\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	// The generated file declares `package examplev1`, so we need to align.
	// We rewrite the package directive for the sandbox build.
	rewrite := strings.Replace(src, "package examplev1", "package sandbox", 1)
	if err := os.WriteFile(filepath.Join(dir, "authn_rules.gen.go"), []byte(rewrite), 0o644); err != nil {
		t.Fatalf("write generated file: %v", err)
	}

	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet failed on generated file:\n%s\n--- source ---\n%s", out, rewrite)
	}
}

// lookupAuthnFile finds the unique authn_rules.gen.go entry produced by the
// plugin, regardless of the directory prefix that protogen derived from
// go_package. Tests that expect exactly one generated file should use this.
func lookupAuthnFile(t *testing.T, files map[string]string) string {
	t.Helper()
	var matches []string
	for k := range files {
		if strings.HasSuffix(k, "/authn_rules.gen.go") || k == "authn_rules.gen.go" {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("expected an authn_rules.gen.go in output, got: %v", keysOf(files))
	}
	if len(matches) > 1 {
		t.Fatalf("expected exactly one authn_rules.gen.go, got: %v", matches)
	}
	return files[matches[0]]
}

// keysOf returns sorted keys for a string-keyed map (debug helper).
func keysOf[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// splitSections splits the generated file at the boundary between the
// _publicMethods declaration and the _methodSchemes declaration so tests can
// assert containment in the correct section without false positives.
func splitSections(t *testing.T, content string) (publicSection, schemesSection string) {
	t.Helper()
	idx := strings.Index(content, "_methodSchemes")
	if idx < 0 {
		return content, ""
	}
	return content[:idx], content[idx:]
}

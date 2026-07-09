package main

import (
	"sort"
	"strings"
	"testing"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// collectDeps walks the FileDescriptor's import closure (and the file itself)
// in topological order, returning each unique file as a FileDescriptorProto.
func collectDeps(fd protoreflect.FileDescriptor) []*descriptorpb.FileDescriptorProto {
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
	_ = protoregistry.GlobalFiles
	return out
}

// methodSpec describes a single RPC entry to materialize on a fake service.
type methodSpec struct {
	name string
	rule *auditv1.AuditRule // nil → no method-level option
}

// serviceSpec describes a single service in the fake proto file.
type serviceSpec struct {
	name           string
	serviceDefault *auditv1.AuditRule
	methods        []methodSpec
}

// fileSpec describes a single proto file to feed the plugin.
type fileSpec struct {
	name     string
	pkg      string
	goPkg    string
	generate bool
	services []serviceSpec
}

// runPluginScenario constructs a fake protogen.Plugin from the given files,
// invokes generate(), and returns the resulting plugin plus any generation error.
func runPluginScenario(t *testing.T, files []fileSpec) (*protogen.Plugin, error) {
	t.Helper()

	deps := collectDeps(auditv1.File_servora_audit_v1_annotations_proto)

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: deps,
	}

	for _, fs := range files {
		fp := buildFileDescriptorProto(t, fs)
		req.ProtoFile = append(req.ProtoFile, fp)
		if fs.generate {
			req.FileToGenerate = append(req.FileToGenerate, fs.name)
		}
	}

	gen, err := protogen.Options{}.New(req)
	if err != nil {
		t.Fatalf("protogen.Options.New: %v", err)
	}

	return gen, generate(gen)
}

func buildFileDescriptorProto(t *testing.T, fs fileSpec) *descriptorpb.FileDescriptorProto {
	t.Helper()

	fp := &descriptorpb.FileDescriptorProto{
		Name:       proto.String(fs.name),
		Package:    proto.String(fs.pkg),
		Syntax:     proto.String(protoreflect.Proto3.String()),
		Dependency: []string{"google/protobuf/descriptor.proto", "servora/audit/v1/annotations.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(fs.goPkg),
		},
	}

	for _, svc := range fs.services {
		sp := &descriptorpb.ServiceDescriptorProto{Name: proto.String(svc.name)}
		if svc.serviceDefault != nil {
			opts := &descriptorpb.ServiceOptions{}
			proto.SetExtension(opts, auditv1.E_ServiceDefault, svc.serviceDefault)
			sp.Options = opts
		}
		for _, m := range svc.methods {
			mp := &descriptorpb.MethodDescriptorProto{
				Name:       proto.String(m.name),
				InputType:  proto.String("." + fs.pkg + ".Empty"),
				OutputType: proto.String("." + fs.pkg + ".Empty"),
			}
			if m.rule != nil {
				opts := &descriptorpb.MethodOptions{}
				proto.SetExtension(opts, auditv1.E_AuditRule, m.rule)
				mp.Options = opts
			}
			sp.Method = append(sp.Method, mp)
		}
		fp.Service = append(fp.Service, sp)
	}

	emptyMsg := &descriptorpb.DescriptorProto{Name: proto.String("Empty")}
	fp.MessageType = append(fp.MessageType, emptyMsg)

	return fp
}

func generatedFiles(t *testing.T, gen *protogen.Plugin) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range gen.Response().File {
		out[f.GetName()] = f.GetContent()
	}
	return out
}

func lookupAuditFile(t *testing.T, files map[string]string) string {
	t.Helper()
	var matches []string
	for k := range files {
		if strings.HasSuffix(k, "/audit_rules.gen.go") || k == "audit_rules.gen.go" {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("expected an audit_rules.gen.go in output, got: %v", keysOf(files))
	}
	if len(matches) > 1 {
		t.Fatalf("expected exactly one audit_rules.gen.go, got: %v", matches)
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

// ── basic behaviour ──────────────────────────────────────────────────────────

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
		t.Fatalf("expected no generated files, got: %v", keysOf(files))
	}
}

func TestMethodLevelEnabled_GoesToOutput(t *testing.T) {
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
						{name: "Hello", rule: &auditv1.AuditRule{
							Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED,
						}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	content := lookupAuditFile(t, generatedFiles(t, gen))

	// Must contain the operation key.
	wantOp := `"/example.v1.GreetingService/Hello"`
	if !strings.Contains(content, wantOp) {
		t.Fatalf("audit rule missing operation key %s\n--- generated ---\n%s", wantOp, content)
	}
	// Must expose the AuditRules function.
	if !strings.Contains(content, "func AuditRules()") {
		t.Errorf("generated code should contain AuditRules()\n--- generated ---\n%s", content)
	}
	// Must set AUDIT_MODE_ENABLED.
	if !strings.Contains(content, "AUDIT_MODE_ENABLED") {
		t.Errorf("generated code should reference AUDIT_MODE_ENABLED\n--- generated ---\n%s", content)
	}
	// Must NOT reference old CompiledRule or BuildEvent.
	if strings.Contains(content, "CompiledRule") {
		t.Errorf("generated code must not reference CompiledRule\n--- generated ---\n%s", content)
	}
	if strings.Contains(content, "BuildEvent") {
		t.Errorf("generated code must not reference BuildEvent\n--- generated ---\n%s", content)
	}
}

func TestMethodLevelDisabled_NotEmitted(t *testing.T) {
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
						{name: "Hello", rule: &auditv1.AuditRule{
							Mode: auditv1.AuditMode_AUDIT_MODE_DISABLED,
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
	if len(files) != 0 {
		t.Fatalf("expected no generated files for DISABLED rule, got: %v", keysOf(files))
	}
}

// ── service default / merge semantics ────────────────────────────────────────

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
					serviceDefault: &auditv1.AuditRule{
						Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED,
					},
					methods: []methodSpec{
						{name: "Hello"}, // no method-level rule → inherits ENABLED
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	content := lookupAuditFile(t, generatedFiles(t, gen))
	wantKey := `"/example.v1.GreetingService/Hello"`
	if !strings.Contains(content, wantKey) {
		t.Fatalf("inherited rule missing operation key %s\n--- generated ---\n%s", wantKey, content)
	}
}

func TestServiceDefault_MethodUnspecifiedInherits(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &auditv1.AuditRule{
						Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED,
					},
					methods: []methodSpec{
						{name: "Hello", rule: &auditv1.AuditRule{
							// Mode UNSPECIFIED → inherits service default.
						}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	content := lookupAuditFile(t, generatedFiles(t, gen))
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Fatalf("expected operation entry for inherited method\n--- generated ---\n%s", content)
	}
}

func TestMethodOverridesServiceDefault_DisabledWins(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &auditv1.AuditRule{
						Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED,
					},
					methods: []methodSpec{
						{name: "Hello"}, // inherits ENABLED
						{name: "Healthz", rule: &auditv1.AuditRule{
							Mode: auditv1.AuditMode_AUDIT_MODE_DISABLED,
						}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	content := lookupAuditFile(t, generatedFiles(t, gen))
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Errorf("Hello should appear (inherits ENABLED)\n--- generated ---\n%s", content)
	}
	if strings.Contains(content, `"/example.v1.GreetingService/Healthz"`) {
		t.Errorf("Healthz should NOT appear (method DISABLED overrides service ENABLED)\n--- generated ---\n%s", content)
	}
}

func TestSameShortServiceNameAcrossPackages_DoesNotShareRules(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "accounts/v1/user.proto",
			pkg:      "accounts.v1",
			goPkg:    "example.com/gen/accounts/v1;accountsv1",
			generate: true,
			services: []serviceSpec{
				{
					name: "UserService",
					serviceDefault: &auditv1.AuditRule{
						Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED,
					},
					methods: []methodSpec{{name: "Get"}},
				},
			},
		},
		{
			name:     "admin/v1/user.proto",
			pkg:      "admin.v1",
			goPkg:    "example.com/gen/admin/v1;adminv1",
			generate: true,
			services: []serviceSpec{
				{
					name: "UserService",
					serviceDefault: &auditv1.AuditRule{
						Mode: auditv1.AuditMode_AUDIT_MODE_DISABLED,
					},
					methods: []methodSpec{{name: "Get"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}

	files := generatedFiles(t, gen)
	accounts := files["example.com/gen/accounts/v1/audit_rules.gen.go"]
	if accounts == "" {
		t.Fatalf("expected generated audit file for accounts package, got: %v", keysOf(files))
	}
	if _, ok := files["example.com/gen/admin/v1/audit_rules.gen.go"]; ok {
		t.Fatalf("admin service is disabled and must not emit rules, got files: %v", keysOf(files))
	}
	if !strings.Contains(accounts, `"/accounts.v1.UserService/Get"`) {
		t.Fatalf("accounts rule missing full-name operation\n--- generated ---\n%s", accounts)
	}
	if strings.Contains(accounts, `"/admin.v1.UserService/Get"`) {
		t.Fatalf("accounts output leaked admin operation\n--- generated ---\n%s", accounts)
	}
}

// ── multi-service / multi-method ─────────────────────────────────────────────

func TestMultipleMethods_AllEnabledPresent(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/svc.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "SvcA",
					methods: []methodSpec{
						{name: "Create", rule: &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED}},
						{name: "Delete", rule: &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED}},
						{name: "Get", rule: &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_DISABLED}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	content := lookupAuditFile(t, generatedFiles(t, gen))
	for _, op := range []string{
		`"/example.v1.SvcA/Create"`,
		`"/example.v1.SvcA/Delete"`,
	} {
		if !strings.Contains(content, op) {
			t.Errorf("expected operation %s in output\n--- generated ---\n%s", op, content)
		}
	}
	if strings.Contains(content, `"/example.v1.SvcA/Get"`) {
		t.Errorf("DISABLED Get should not appear\n--- generated ---\n%s", content)
	}
}

func TestGeneratedFile_HasCorrectHeader(t *testing.T) {
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/svc.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "Svc",
					methods: []methodSpec{
						{name: "Op", rule: &auditv1.AuditRule{Mode: auditv1.AuditMode_AUDIT_MODE_ENABLED}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	content := lookupAuditFile(t, generatedFiles(t, gen))
	if !strings.Contains(content, "Code generated by protoc-gen-servora-audit") {
		t.Errorf("missing generated-code header\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "DO NOT EDIT") {
		t.Errorf("missing DO NOT EDIT marker\n--- generated ---\n%s", content)
	}
}

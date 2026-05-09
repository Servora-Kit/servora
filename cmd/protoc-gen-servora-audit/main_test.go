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
// This lets the synthetic CodeGeneratorRequest include every transitive
// dependency required by protogen to resolve type references (e.g.
// google/protobuf/timestamp.proto when audit.proto imports it).
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
	_ = protoregistry.GlobalFiles // keep package referenced even if visit body grows
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
	serviceDefault *auditv1.AuditRule // nil → no service-level default
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
// invokes generate(), and returns the resulting plugin (so tests can inspect
// generated files) plus any generation error.
func runPluginScenario(t *testing.T, files []fileSpec) (*protogen.Plugin, error) {
	t.Helper()

	// Walk the audit annotations.proto import closure so transitive deps
	// (audit.proto → google/protobuf/timestamp.proto, descriptor.proto, …)
	// land in the request in topological order.
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

	// Inject Empty placeholder so InputType/OutputType refs resolve.
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

// lookupAuditFile finds the unique audit_rules.gen.go entry in the generated
// output. Tests that expect exactly one generated file should use this.
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
							Mode:         auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType:    auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
							MutationType: auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_CREATE,
							TargetType:   "greeting",
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
	content := lookupAuditFile(t, files)
	wantOp := `"/example.v1.GreetingService/Hello"`
	if !strings.Contains(content, wantOp) {
		t.Fatalf("audit rule missing operation key %s\n--- generated ---\n%s", wantOp, content)
	}
	// gofmt aligns map-literal field names to the longest sibling, so the
	// number of spaces after `TargetType:` depends on neighbouring fields.
	// Match the field name and value separately to stay format-tolerant.
	if !strings.Contains(content, "TargetType:") || !strings.Contains(content, `"greeting"`) {
		t.Errorf("audit rule missing TargetType literal\n--- generated ---\n%s", content)
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
						Mode:       auditv1.AuditMode_AUDIT_MODE_ENABLED,
						EventType:  auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
						TargetType: "greeting",
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
	files := generatedFiles(t, gen)
	content := lookupAuditFile(t, files)
	wantKey := `"/example.v1.GreetingService/Hello"`
	if !strings.Contains(content, wantKey) {
		t.Fatalf("inherited rule missing operation key %s\n--- generated ---\n%s", wantKey, content)
	}
	// Match field name + value separately to tolerate gofmt alignment.
	if !strings.Contains(content, "TargetType:") || !strings.Contains(content, `"greeting"`) {
		t.Errorf("inherited rule missing TargetType literal\n--- generated ---\n%s", content)
	}
}

func TestServiceDefault_MethodUnspecifiedInherits(t *testing.T) {
	// Method-level rule exists but mode==UNSPECIFIED — should still inherit.
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
						Mode:       auditv1.AuditMode_AUDIT_MODE_ENABLED,
						EventType:  auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
						TargetType: "greeting",
					},
					methods: []methodSpec{
						{name: "Hello", rule: &auditv1.AuditRule{
							// Mode left UNSPECIFIED — inherits service default.
							TargetType: "ignored-because-mode-unspecified",
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
	content := lookupAuditFile(t, files)
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Fatalf("expected operation entry for inherited method\n--- generated ---\n%s", content)
	}
	// The inherited rule should carry the service default's TargetType, not
	// the method-level "ignored" value (because UNSPECIFIED inherits whole).
	if !strings.Contains(content, "TargetType:") || !strings.Contains(content, `"greeting"`) {
		t.Errorf("inherited TargetType lost (or method-level partial leaked)\n--- generated ---\n%s", content)
	}
	if strings.Contains(content, "ignored-because-mode-unspecified") {
		t.Errorf("method-level partial leaked into output (UNSPECIFIED should inherit whole rule)\n--- generated ---\n%s", content)
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
						Mode:       auditv1.AuditMode_AUDIT_MODE_ENABLED,
						EventType:  auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
						TargetType: "greeting",
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
	files := generatedFiles(t, gen)
	content := lookupAuditFile(t, files)
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Errorf("Hello should appear (inherits ENABLED)\n--- generated ---\n%s", content)
	}
	if strings.Contains(content, `"/example.v1.GreetingService/Healthz"`) {
		t.Errorf("Healthz should NOT appear (method DISABLED overrides ENABLED service default)\n--- generated ---\n%s", content)
	}
}

func TestRecordOnError_EnumOnFoldsToTrue(t *testing.T) {
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
							Mode:          auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType:     auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
							TargetType:    "greeting",
							RecordOnError: auditv1.ErrorRecordMode_ERROR_RECORD_MODE_ON,
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
	if !strings.Contains(content, "RecordOnError: true") {
		t.Errorf("ERROR_RECORD_MODE_ON should fold to RecordOnError: true literal\n--- generated ---\n%s", content)
	}
}

func TestRecordOnError_EnumOffFoldsToFalse(t *testing.T) {
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
							Mode:          auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType:     auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
							TargetType:    "greeting",
							RecordOnError: auditv1.ErrorRecordMode_ERROR_RECORD_MODE_OFF,
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
	if strings.Contains(content, "RecordOnError: true") {
		t.Errorf("ERROR_RECORD_MODE_OFF should NOT emit RecordOnError: true\n--- generated ---\n%s", content)
	}
}

func TestRecordOnError_EnumUnspecifiedFoldsToFalse(t *testing.T) {
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
							Mode:       auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType:  auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION,
							TargetType: "greeting",
							// RecordOnError left UNSPECIFIED.
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
	if strings.Contains(content, "RecordOnError: true") {
		t.Errorf("UNSPECIFIED record_on_error must not emit RecordOnError: true\n--- generated ---\n%s", content)
	}
}

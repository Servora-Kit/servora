package main

import (
	"sort"
	"strings"
	"testing"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	cev1 "github.com/Servora-Kit/servora/api/gen/go/servora/cloudevents/v1"
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
	// (audit.proto → cloudevents.proto → google/protobuf/timestamp.proto, …)
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
							Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType: "servora.audit.resource_mutation",
							Severity:  "INFO",
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
	if !strings.Contains(content, `EventType: "servora.audit.resource_mutation"`) {
		t.Errorf("audit rule missing EventType literal\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, `"INFO"`) {
		t.Errorf("audit rule missing Severity literal\n--- generated ---\n%s", content)
	}
	// Must have CompiledRule reference.
	if !strings.Contains(content, "CompiledRule") {
		t.Errorf("generated code should reference audit.CompiledRule\n--- generated ---\n%s", content)
	}
	// Must have BuildEvent function.
	if !strings.Contains(content, "BuildEvent:") {
		t.Errorf("generated code should include BuildEvent field\n--- generated ---\n%s", content)
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
						Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
						EventType: "servora.audit.resource_mutation",
						Severity:  "INFO",
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
	if !strings.Contains(content, `"servora.audit.resource_mutation"`) {
		t.Errorf("inherited rule missing EventType\n--- generated ---\n%s", content)
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
						Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
						EventType: "servora.audit.resource_mutation",
						Severity:  "INFO",
					},
					methods: []methodSpec{
						{name: "Hello", rule: &auditv1.AuditRule{
							// Mode left UNSPECIFIED — inherits service default.
							EventType: "ignored-because-mode-unspecified",
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
	// The inherited rule should carry the service default's EventType, not
	// the method-level "ignored" value (because UNSPECIFIED inherits whole).
	if !strings.Contains(content, `"servora.audit.resource_mutation"`) {
		t.Errorf("inherited EventType lost (or method-level partial leaked)\n--- generated ---\n%s", content)
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
						Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
						EventType: "servora.audit.resource_mutation",
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

func TestTargetIDField_GeneratesSetSubject(t *testing.T) {
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
							EventType:     "servora.audit.resource_mutation",
							TargetIdField: "resp.id",
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
	if !strings.Contains(content, "SetSubject") {
		t.Errorf("expected SetSubject call for target_id_field\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "GetId()") {
		t.Errorf("expected GetId() getter chain for target_id_field=resp.id\n--- generated ---\n%s", content)
	}
}

func TestDetailMessageField_GeneratesSetProtoData(t *testing.T) {
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
							Mode:               auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType:          "servora.audit.resource_mutation",
							DetailMessageField: "req",
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
	if !strings.Contains(content, "SetProtoData") {
		t.Errorf("expected SetProtoData call for detail_message_field\n--- generated ---\n%s", content)
	}
}

func TestExtensionsLiteral_GeneratesSetExtension(t *testing.T) {
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
							Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType: "servora.audit.resource_mutation",
							Extensions: []*auditv1.ExtensionMapping{
								{
									Name: "mutation",
									Source: &auditv1.ExtensionMapping_Literal{
										Literal: &cev1.CloudEvent_CloudEventAttributeValue{
											Attr: &cev1.CloudEvent_CloudEventAttributeValue_CeString{
												CeString: "CREATE",
											},
										},
									},
								},
								{
									Name: "resourcetype",
									Source: &auditv1.ExtensionMapping_Literal{
										Literal: &cev1.CloudEvent_CloudEventAttributeValue{
											Attr: &cev1.CloudEvent_CloudEventAttributeValue_CeString{
												CeString: "greeting",
											},
										},
									},
								},
							},
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
	if !strings.Contains(content, `"mutation"`) {
		t.Errorf("expected extension name 'mutation' in output\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, `"CREATE"`) {
		t.Errorf("expected literal value 'CREATE' in output\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, `"resourcetype"`) {
		t.Errorf("expected extension name 'resourcetype' in output\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, `"greeting"`) {
		t.Errorf("expected literal value 'greeting' in output\n--- generated ---\n%s", content)
	}
}

func TestExtensionsFromField_GeneratesFieldAccess(t *testing.T) {
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
							Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType: "servora.audit.resource_mutation",
							Extensions: []*auditv1.ExtensionMapping{
								{
									Name: "tenant",
									Source: &auditv1.ExtensionMapping_FromField{
										FromField: "req.tenant_id",
									},
								},
							},
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
	if !strings.Contains(content, `"tenant"`) {
		t.Errorf("expected extension name 'tenant' in output\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "GetTenantId()") {
		t.Errorf("expected GetTenantId() getter for from_field=req.tenant_id\n--- generated ---\n%s", content)
	}
}

func TestErrorHandling_GeneratesErrorBlock(t *testing.T) {
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
							Mode:      auditv1.AuditMode_AUDIT_MODE_ENABLED,
							EventType: "servora.audit.resource_mutation",
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
	// Every BuildEvent should include error handling with ExtSeverityText and ExtErrorMessage.
	if !strings.Contains(content, "err != nil") {
		t.Errorf("expected error check in BuildEvent\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "ExtSeverityText") {
		t.Errorf("expected ExtSeverityText constant usage\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "ExtErrorMessage") {
		t.Errorf("expected ExtErrorMessage constant usage\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, `"ERROR"`) {
		t.Errorf("expected ERROR severity on error path\n--- generated ---\n%s", content)
	}
}

func TestNestedTargetIDField(t *testing.T) {
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
							EventType:     "servora.audit.resource_mutation",
							TargetIdField: "resp.user.id",
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
	if !strings.Contains(content, "GetUser().GetId()") {
		t.Errorf("expected chained getters GetUser().GetId() for resp.user.id\n--- generated ---\n%s", content)
	}
}

func TestBuildGetterChain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id", "GetId()"},
		{"user_id", "GetUserId()"},
		{"user.id", "GetUser().GetId()"},
		{"user.tenant_id", "GetUser().GetTenantId()"},
	}
	for _, tc := range tests {
		got := buildGetterChain(tc.input)
		if got != tc.want {
			t.Errorf("buildGetterChain(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSnakeToPascal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id", "Id"},
		{"user_id", "UserId"},
		{"tenant_id", "TenantId"},
		{"some_long_name", "SomeLongName"},
	}
	for _, tc := range tests {
		got := snakeToPascal(tc.input)
		if got != tc.want {
			t.Errorf("snakeToPascal(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

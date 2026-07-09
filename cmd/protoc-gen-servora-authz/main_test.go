package main

import (
	"sort"
	"strings"
	"testing"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	return out
}

type methodSpec struct {
	name string
	rule *authzpb.AuthzRule // nil → no method-level option
}

type serviceSpec struct {
	name           string
	serviceDefault *authzpb.AuthzRule // nil → no service-level default
	methods        []methodSpec
}

type fileSpec struct {
	name     string
	pkg      string
	goPkg    string
	generate bool
	services []serviceSpec
}

func runPluginScenario(t *testing.T, files []fileSpec) (*protogen.Plugin, error) {
	t.Helper()

	deps := collectDeps(authzpb.File_servora_authz_v1_authz_proto)

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
		Dependency: []string{"google/protobuf/descriptor.proto", "servora/authz/v1/authz.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(fs.goPkg),
		},
	}

	for _, svc := range fs.services {
		sp := &descriptorpb.ServiceDescriptorProto{Name: proto.String(svc.name)}
		if svc.serviceDefault != nil {
			opts := &descriptorpb.ServiceOptions{}
			proto.SetExtension(opts, authzpb.E_ServiceDefault, svc.serviceDefault)
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
				proto.SetExtension(opts, authzpb.E_Rule, m.rule)
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

func lookupAuthzFile(t *testing.T, files map[string]string) string {
	t.Helper()
	var matches []string
	for k := range files {
		if strings.HasSuffix(k, "/authz_rules.gen.go") || k == "authz_rules.gen.go" {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("expected an authz_rules.gen.go in output, got: %v", keysOf(files))
	}
	if len(matches) > 1 {
		t.Fatalf("expected exactly one authz_rules.gen.go, got: %v", matches)
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

func TestMethodLevelCheck_GoesToOutput(t *testing.T) {
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
						{name: "Hello", rule: &authzpb.AuthzRule{
							Mode:         authzpb.AuthzMode_AUTHZ_MODE_CHECK,
							Action:       "user",
							ResourceType: "greeting",
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
	content := lookupAuthzFile(t, files)
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Fatalf("authz rule missing operation key\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "AuthzMode_AUTHZ_MODE_CHECK") {
		t.Errorf("authz rule missing CHECK mode literal\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "Action:") || !strings.Contains(content, `"user"`) {
		t.Errorf("authz rule missing Action literal\n--- generated ---\n%s", content)
	}
}

func TestServiceDefault_MethodInherits(t *testing.T) {
	// Service default CHECK with no method-level rule → method inherits.
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authzpb.AuthzRule{
						Mode:         authzpb.AuthzMode_AUTHZ_MODE_CHECK,
						Action:       "user",
						ResourceType: "greeting",
					},
					methods: []methodSpec{
						{name: "Hello"}, // no method-level rule → inherits CHECK
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate returned unexpected error: %v", err)
	}
	files := generatedFiles(t, gen)
	content := lookupAuthzFile(t, files)
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Fatalf("inherited rule missing operation key\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "AuthzMode_AUTHZ_MODE_CHECK") {
		t.Errorf("inherited mode lost\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "Action:") || !strings.Contains(content, `"user"`) {
		t.Errorf("inherited Action lost\n--- generated ---\n%s", content)
	}
}

func TestServiceDefault_MethodUnspecifiedInherits(t *testing.T) {
	// Method-level rule with mode==UNSPECIFIED still inherits service default
	// in its entirety (no partial merge).
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authzpb.AuthzRule{
						Mode:         authzpb.AuthzMode_AUTHZ_MODE_CHECK,
						Action:       "user",
						ResourceType: "greeting",
					},
					methods: []methodSpec{
						{name: "Hello", rule: &authzpb.AuthzRule{
							// Mode left UNSPECIFIED — partial fields like
							// Action should NOT leak into the merged rule.
							Action: "ignored-because-mode-unspecified",
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
	content := lookupAuthzFile(t, files)
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Fatalf("inherited rule missing operation key\n--- generated ---\n%s", content)
	}
	if !strings.Contains(content, "Action:") || !strings.Contains(content, `"user"`) {
		t.Errorf("inherited Action lost (or method partial leaked)\n--- generated ---\n%s", content)
	}
	if strings.Contains(content, "ignored-because-mode-unspecified") {
		t.Errorf("method-level partial leaked into output (UNSPECIFIED should inherit whole)\n--- generated ---\n%s", content)
	}
}

func TestMethodOverridesServiceDefault_NoneWins(t *testing.T) {
	// Service default CHECK; one method declares NONE explicitly → that
	// method's emitted rule has mode NONE, not CHECK.
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/greeting.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "GreetingService",
					serviceDefault: &authzpb.AuthzRule{
						Mode:         authzpb.AuthzMode_AUTHZ_MODE_CHECK,
						Action:       "user",
						ResourceType: "greeting",
					},
					methods: []methodSpec{
						{name: "Hello"}, // inherits CHECK
						{name: "Healthz", rule: &authzpb.AuthzRule{
							Mode: authzpb.AuthzMode_AUTHZ_MODE_NONE,
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
	content := lookupAuthzFile(t, files)

	// Hello should be CHECK (inherited).
	if !strings.Contains(content, `"/example.v1.GreetingService/Hello"`) {
		t.Errorf("Hello entry missing\n--- generated ---\n%s", content)
	}
	// Healthz should be present with NONE (override won outright; service
	// default's action/resource_type must not leak).
	healthzMarker := `"/example.v1.GreetingService/Healthz"`
	if !strings.Contains(content, healthzMarker) {
		t.Fatalf("Healthz entry missing\n--- generated ---\n%s", content)
	}
	// Slice the file at Healthz key and check the next few lines.
	idx := strings.Index(content, healthzMarker)
	tail := content[idx:]
	end := strings.Index(tail, "},")
	if end < 0 {
		end = len(tail)
	}
	healthzBlock := tail[:end]
	if !strings.Contains(healthzBlock, "AuthzMode_AUTHZ_MODE_NONE") {
		t.Errorf("Healthz should carry NONE mode (override), got block:\n%s", healthzBlock)
	}
	// Override drops service-default fields entirely; the Healthz block
	// should contain neither an Action entry nor the "user" action value.
	if strings.Contains(healthzBlock, "Action:") || strings.Contains(healthzBlock, `"user"`) {
		t.Errorf("Healthz override should drop service-default Action, got block:\n%s", healthzBlock)
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
					serviceDefault: &authzpb.AuthzRule{
						Mode:         authzpb.AuthzMode_AUTHZ_MODE_CHECK,
						Action:       "read",
						ResourceType: "account_user",
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
					serviceDefault: &authzpb.AuthzRule{
						Mode:         authzpb.AuthzMode_AUTHZ_MODE_CHECK,
						Action:       "admin_read",
						ResourceType: "admin_user",
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
	accounts := files["example.com/gen/accounts/v1/authz_rules.gen.go"]
	admin := files["example.com/gen/admin/v1/authz_rules.gen.go"]
	if accounts == "" || admin == "" {
		t.Fatalf("expected generated authz files for both packages, got: %v", keysOf(files))
	}
	if !strings.Contains(accounts, `"/accounts.v1.UserService/Get"`) {
		t.Fatalf("accounts rule missing full-name operation\n--- generated ---\n%s", accounts)
	}
	if strings.Contains(accounts, `"/admin.v1.UserService/Get"`) ||
		strings.Contains(accounts, `"admin_read"`) ||
		strings.Contains(accounts, `"admin_user"`) {
		t.Fatalf("accounts output leaked admin rule\n--- generated ---\n%s", accounts)
	}
	if !strings.Contains(accounts, `"read"`) || !strings.Contains(accounts, `"account_user"`) {
		t.Fatalf("accounts rule lost its own fields\n--- generated ---\n%s", accounts)
	}
	if !strings.Contains(admin, `"/admin.v1.UserService/Get"`) {
		t.Fatalf("admin rule missing full-name operation\n--- generated ---\n%s", admin)
	}
	if strings.Contains(admin, `"/accounts.v1.UserService/Get"`) ||
		strings.Contains(admin, `"account_user"`) ||
		strings.Contains(admin, `"read",`) {
		t.Fatalf("admin output leaked accounts rule\n--- generated ---\n%s", admin)
	}
	if !strings.Contains(admin, `"admin_read"`) || !strings.Contains(admin, `"admin_user"`) {
		t.Fatalf("admin rule lost its own fields\n--- generated ---\n%s", admin)
	}
}

func TestServiceDefault_NoMethodsDeclared_NoOutput(t *testing.T) {
	// Service-level default is set but the service has no methods declared.
	// No rule should be emitted (nothing to bind to).
	gen, err := runPluginScenario(t, []fileSpec{
		{
			name:     "example/v1/empty.proto",
			pkg:      "example.v1",
			goPkg:    "example.com/gen/example/v1;examplev1",
			generate: true,
			services: []serviceSpec{
				{
					name: "EmptyService",
					serviceDefault: &authzpb.AuthzRule{
						Mode:   authzpb.AuthzMode_AUTHZ_MODE_CHECK,
						Action: "user",
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
		t.Fatalf("expected no generated files for service with no methods, got: %v", keysOf(files))
	}
}

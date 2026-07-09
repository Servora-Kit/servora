// Command protoc-gen-servora-authz translates servora authz proto annotations
// into a Go file (`authz_rules.gen.go`) consumed by the runtime to enforce
// authorization on RPC methods.
//
// Merge semantics (matches authn / audit):
//   - method-level rule with mode != AUTHZ_MODE_UNSPECIFIED replaces the
//     service-level default in its entirety,
//   - method-level rule absent (or mode == AUTHZ_MODE_UNSPECIFIED) inherits
//     the service-level default,
//   - only methods whose merged mode != AUTHZ_MODE_UNSPECIFIED appear in the
//     generated map (NONE is preserved so callers can express "explicitly
//     skip" rather than "no rule"; the runtime decides what to do with NONE).
//
// Cross-file template scanning: rules are gathered from ALL input files
// (including non-generated dependencies). Templates are keyed by proto
// fully-qualified service name so same-named services in different packages do
// not share rules. Generated output groups by output directory so each
// directory yields one authz_rules.gen.go covering the services declared in it.
package main

import (
	"fmt"
	"path"
	"sort"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
	"github.com/Servora-Kit/servora/cmd/internal/optionmerge"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return generate(gen)
	})
}

// ruleParams holds the merged authorization parameters for a single method.
type ruleParams struct {
	Mode            authzpb.AuthzMode
	Action          string
	ResourceType    string
	ResourceIdField string
}

type ruleEntry struct {
	Operation       string
	Mode            authzpb.AuthzMode
	Action          string
	ResourceType    string
	ResourceIdField string
}

// generate is the testable entry point.
func generate(gen *protogen.Plugin) error {
	// First pass: build cross-file template index.
	//   - serviceDefaults[fullName] → service-level default rule
	//   - methodTemplates[fullName][methodName] → method-level rule
	//
	// Scanning all files (not just gen.Files) keeps generated files stable when
	// a generated target depends on another annotated proto. The full service
	// name is the identity boundary; short service names are not globally
	// unique across proto packages.
	serviceDefaults := map[string]*authzpb.AuthzRule{}
	methodTemplates := map[string]map[string]*authzpb.AuthzRule{}
	for _, f := range gen.Files {
		for _, svc := range f.Services {
			fullName := string(svc.Desc.FullName())

			if def := extractServiceDefault(svc); def != nil {
				serviceDefaults[fullName] = def
			}

			for _, m := range svc.Methods {
				if r := extractMethodRule(m); r != nil {
					if methodTemplates[fullName] == nil {
						methodTemplates[fullName] = map[string]*authzpb.AuthzRule{}
					}
					methodTemplates[fullName][string(m.Desc.Name())] = r
				}
			}
		}
	}

	// Second pass: emit one authz_rules.gen.go per directory that has at
	// least one resolved rule. Resolve every method of every service in a
	// generated file by merging service default + method rule.
	type dirGroup struct {
		targetFile *protogen.File
		seen       map[string]bool
		rules      []ruleEntry
	}
	groups := map[string]*dirGroup{}

	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		dir := path.Dir(f.GeneratedFilenamePrefix)
		for _, svc := range f.Services {
			fullName := string(svc.Desc.FullName())

			svcDefault := serviceDefaults[fullName]
			methods := methodTemplates[fullName]

			// Iterate every declared method of this service, not just the
			// ones with method-level annotations — methods inheriting the
			// service default need to land in the output too.
			for _, m := range svc.Methods {
				methodName := string(m.Desc.Name())
				methodR := methods[methodName]
				merged, ok := optionmerge.Merge(svcDefault, methodR, methodR != nil)
				if !ok {
					continue
				}
				p := ruleParams{
					Mode:            merged.Mode,
					Action:          merged.GetAction(),
					ResourceType:    merged.GetResourceType(),
					ResourceIdField: merged.GetResourceIdField(),
				}

				if groups[dir] == nil {
					groups[dir] = &dirGroup{
						targetFile: f,
						seen:       map[string]bool{},
					}
				}

				op := fmt.Sprintf("/%s/%s", fullName, methodName)
				if groups[dir].seen[op] {
					continue
				}
				groups[dir].seen[op] = true
				groups[dir].rules = append(groups[dir].rules, ruleEntry{
					Operation:       op,
					Mode:            p.Mode,
					Action:          p.Action,
					ResourceType:    p.ResourceType,
					ResourceIdField: p.ResourceIdField,
				})
			}
		}
	}

	if len(groups) == 0 {
		return nil
	}

	dirs := make([]string, 0, len(groups))
	for d := range groups {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		group := groups[dir]
		sort.Slice(group.rules, func(i, j int) bool {
			return group.rules[i].Operation < group.rules[j].Operation
		})
		g := gen.NewGeneratedFile(
			path.Join(dir, "authz_rules.gen.go"),
			group.targetFile.GoImportPath,
		)
		generateFile(g, group.targetFile.GoPackageName, group.rules)
	}

	return nil
}

// extractMethodRule returns the AuthzRule attached to a method via E_Rule, or
// nil when no extension is set.
func extractMethodRule(m *protogen.Method) *authzpb.AuthzRule {
	opts := m.Desc.Options()
	if opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, authzpb.E_Rule) {
		return nil
	}
	r, ok := proto.GetExtension(opts, authzpb.E_Rule).(*authzpb.AuthzRule)
	if !ok || r == nil {
		return nil
	}
	return r
}

// extractServiceDefault returns the service-level default rule via
// E_ServiceDefault, or nil when no extension is set.
func extractServiceDefault(s *protogen.Service) *authzpb.AuthzRule {
	opts := s.Desc.Options()
	if opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, authzpb.E_ServiceDefault) {
		return nil
	}
	r, ok := proto.GetExtension(opts, authzpb.E_ServiceDefault).(*authzpb.AuthzRule)
	if !ok || r == nil {
		return nil
	}
	return r
}

// mergeRules is now provided by cmd/internal/optionmerge.Merge.

func generateFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, rules []ruleEntry) {
	authzPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1")
	protoPkg := protogen.GoImportPath("google.golang.org/protobuf/proto")

	g.P("// Code generated by protoc-gen-servora-authz. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	authzRule := g.QualifiedGoIdent(protogen.GoIdent{GoName: "AuthzRule", GoImportPath: authzPkg})
	protoClone := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Clone", GoImportPath: protoPkg})

	g.P("// _authzRules is the immutable backing store for AuthzRules.")
	g.P("var _authzRules = map[string]*", authzRule, "{")
	for _, r := range rules {
		modeIdent := g.QualifiedGoIdent(protogen.GoIdent{
			GoName:       "AuthzMode_" + r.Mode.String(),
			GoImportPath: authzPkg,
		})
		g.P(fmt.Sprintf("	%q: {", r.Operation))
		g.P("		Mode: ", modeIdent, ",")
		if r.Action != "" {
			g.P(fmt.Sprintf("		Action: %q,", r.Action))
		}
		if r.ResourceType != "" {
			g.P(fmt.Sprintf("		ResourceType: %q,", r.ResourceType))
		}
		if r.ResourceIdField != "" {
			g.P(fmt.Sprintf("		ResourceIdField: %q,", r.ResourceIdField))
		}
		g.P("	},")
	}
	g.P("}")
	g.P()
	g.P("// AuthzRules returns a copy of the authorization rules for this package.")
	g.P("// Each call returns a fresh map and cloned rule messages.")
	g.P("func AuthzRules() map[string]*", authzRule, " {")
	g.P("	m := make(map[string]*", authzRule, ", len(_authzRules))")
	g.P("	for k, v := range _authzRules {")
	g.P("		m[k] = ", protoClone, "(v).(*", authzRule, ")")
	g.P("	}")
	g.P("	return m")
	g.P("}")
}

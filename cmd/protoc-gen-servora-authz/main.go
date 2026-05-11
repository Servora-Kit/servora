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
// (including non-generated dependencies) so authz annotations on canonical
// RPC protos remain visible when only their HTTP-gateway counterparts are in
// the generation set. Generated output groups by output directory so each
// directory yields one authz_rules.gen.go covering the services declared in
// it (resolved through the cross-file template index).
package main

import (
	"fmt"
	"path"
	"sort"

	authzpb "github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1"
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
	ResourceIDField string
}

type ruleEntry struct {
	Operation       string
	Mode            authzpb.AuthzMode
	Action          string
	ResourceType    string
	ResourceIDField string
}

// generate is the testable entry point.
func generate(gen *protogen.Plugin) error {
	// First pass: build cross-file template index.
	//   - serviceDefaults[shortName] → service-level default rule
	//   - methodTemplates[shortName][methodName] → method-level rule
	//
	// Scanning all files (not just gen.Files) lets HTTP-gateway protos that
	// re-declare a service inherit the annotations from their pure-RPC
	// counterpart, which is included in the request as a dependency.
	serviceDefaults := map[string]*authzpb.AuthzRule{}
	methodTemplates := map[string]map[string]*authzpb.AuthzRule{}
	for _, f := range gen.Files {
		for _, svc := range f.Services {
			shortName := string(svc.Desc.Name())

			if def := extractServiceDefault(svc); def != nil {
				// Last writer wins if the same short name appears in multiple
				// files — matches the existing method-level merging pattern.
				serviceDefaults[shortName] = def
			}

			for _, m := range svc.Methods {
				if r := extractMethodRule(m); r != nil {
					if methodTemplates[shortName] == nil {
						methodTemplates[shortName] = map[string]*authzpb.AuthzRule{}
					}
					methodTemplates[shortName][string(m.Desc.Name())] = r
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
			shortName := string(svc.Desc.Name())
			fullName := string(svc.Desc.FullName())

			svcDefault := serviceDefaults[shortName]
			methods := methodTemplates[shortName]

			// Iterate every declared method of this service, not just the
			// ones with method-level annotations — methods inheriting the
			// service default need to land in the output too.
			for _, m := range svc.Methods {
				methodName := string(m.Desc.Name())
				p, ok := mergeRules(svcDefault, methods[methodName])
				if !ok {
					continue
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
					ResourceIDField: p.ResourceIDField,
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

// mergeRules implements the spec's merge semantics. Returns (params, true)
// when the method should be emitted; (_, false) otherwise.
//
// A method-level rule with mode != UNSPECIFIED fully replaces the service
// default. UNSPECIFIED (or absence) inherits the service default verbatim. If
// neither side contributes a non-UNSPECIFIED mode, the method is skipped.
func mergeRules(svcDefault, methodRule *authzpb.AuthzRule) (ruleParams, bool) {
	if methodRule != nil && methodRule.Mode != authzpb.AuthzMode_AUTHZ_MODE_UNSPECIFIED {
		return ruleParams{
			Mode:            methodRule.Mode,
			Action:          methodRule.GetAction(),
			ResourceType:    methodRule.GetResourceType(),
			ResourceIDField: methodRule.GetResourceIdField(),
		}, true
	}
	if svcDefault != nil && svcDefault.Mode != authzpb.AuthzMode_AUTHZ_MODE_UNSPECIFIED {
		return ruleParams{
			Mode:            svcDefault.Mode,
			Action:          svcDefault.GetAction(),
			ResourceType:    svcDefault.GetResourceType(),
			ResourceIDField: svcDefault.GetResourceIdField(),
		}, true
	}
	return ruleParams{}, false
}

func generateFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, rules []ruleEntry) {
	authzPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/api/gen/go/servora/authz/v1")
	pkgAuthzPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/security/authz")

	g.P("// Code generated by protoc-gen-servora-authz. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	authzRule := g.QualifiedGoIdent(protogen.GoIdent{GoName: "AuthzRule", GoImportPath: pkgAuthzPkg})

	g.P("// _authzRules is the immutable backing store for AuthzRules.")
	g.P("var _authzRules = map[string]", authzRule, "{")
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
		if r.ResourceIDField != "" {
			g.P(fmt.Sprintf("		ResourceIDField: %q,", r.ResourceIDField))
		}
		g.P("	},")
	}
	g.P("}")
	g.P()
	g.P("// AuthzRules returns a copy of the authorization rules for this package.")
	g.P("// Each call returns an independent map; callers may not modify the returned map.")
	g.P("func AuthzRules() map[string]", authzRule, " {")
	g.P("	m := make(map[string]", authzRule, ", len(_authzRules))")
	g.P("	for k, v := range _authzRules {")
	g.P("		m[k] = v")
	g.P("	}")
	g.P("	return m")
	g.P("}")
}

// Command protoc-gen-servora-authn translates servora authn proto annotations
// into a Go file (`authn_rules.gen.go`) that the runtime can consult to decide
// which RPC methods are public, and which method requires which authentication
// schemes.
//
// Merge semantics:
//   - method-level rule with mode != MODE_UNSPECIFIED replaces the service
//     default in its entirety (schemes from the service default are dropped),
//   - method-level rule absent (or mode == MODE_UNSPECIFIED) inherits the
//     service-level default,
//   - if neither is declared, the method does not appear in any output.
//
// Validation (any failure aborts code generation):
//   - mode == MODE_UNSPECIFIED with non-empty schemes,
//   - mode == MODE_PUBLIC with non-empty schemes (mutually exclusive).
package main

import (
	"fmt"
	"path"
	"sort"

	authnpb "github.com/Servora-Kit/servora/api/gen/go/servora/authn/v1"
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

// generate is the testable entry point. It scans every file in gen.Files,
// computes the merged authn rules, validates the result, and writes
// authn_rules.gen.go into each output directory that has at least one rule.
func generate(gen *protogen.Plugin) error {
	serviceDefaults := map[string]*authnpb.AuthnRule{}
	methodTemplates := map[string]map[string]*authnpb.AuthnRule{}
	for _, f := range gen.Files {
		for _, svc := range f.Services {
			fullName := string(svc.Desc.FullName())

			if def := serviceDefault(svc); def != nil {
				if err := validateRule(f.Desc.Path(), fullName, "<service_default>", def); err != nil {
					return err
				}
				serviceDefaults[fullName] = def
			}

			for _, m := range svc.Methods {
				rule, hasMethod := methodRule(m)
				if !hasMethod {
					continue
				}
				if err := validateRule(f.Desc.Path(), fullName, string(m.Desc.Name()), rule); err != nil {
					return err
				}
				if methodTemplates[fullName] == nil {
					methodTemplates[fullName] = map[string]*authnpb.AuthnRule{}
				}
				methodTemplates[fullName][string(m.Desc.Name())] = rule
			}
		}
	}

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

			for _, m := range svc.Methods {
				methodName := string(m.Desc.Name())
				rule, hasMethod := methods[methodName]
				merged, ok := optionmerge.Merge(svcDefault, rule, hasMethod)
				if !ok {
					continue
				}

				if err := validateRule(f.Desc.Path(), fullName, methodName, merged); err != nil {
					return err
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
					Operation: op,
					Mode:      merged.GetMode(),
					Schemes:   append([]string(nil), merged.GetSchemes()...),
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
		g := groups[dir]
		sort.Slice(g.rules, func(i, j int) bool {
			return g.rules[i].Operation < g.rules[j].Operation
		})

		gf := gen.NewGeneratedFile(
			path.Join(dir, "authn_rules.gen.go"),
			g.targetFile.GoImportPath,
		)
		writeFile(gf, g.targetFile.GoPackageName, g.rules)
	}

	return nil
}

type ruleEntry struct {
	Operation string
	Mode      authnpb.AuthnRule_Mode
	Schemes   []string
}

// methodRule extracts the AuthnRule attached to a method via E_Rule, returning
// hasMethod == true only when the extension is present.
func methodRule(m *protogen.Method) (*authnpb.AuthnRule, bool) {
	opts := m.Desc.Options()
	if opts == nil {
		return nil, false
	}
	if !proto.HasExtension(opts, authnpb.E_Rule) {
		return nil, false
	}
	r, ok := proto.GetExtension(opts, authnpb.E_Rule).(*authnpb.AuthnRule)
	if !ok || r == nil {
		return nil, false
	}
	return r, true
}

// serviceDefault extracts the service-level default (E_ServiceDefault), or nil.
func serviceDefault(s *protogen.Service) *authnpb.AuthnRule {
	opts := s.Desc.Options()
	if opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, authnpb.E_ServiceDefault) {
		return nil
	}
	r, ok := proto.GetExtension(opts, authnpb.E_ServiceDefault).(*authnpb.AuthnRule)
	if !ok || r == nil {
		return nil
	}
	return r
}

// mergeRules is now provided by cmd/internal/optionmerge.Merge.

// validateRule rejects illegal mode/schemes combinations. The error message
// always includes the file path, service name, method name, and the violation
// (so business owners can find the offending annotation quickly).
func validateRule(file, service, method string, r *authnpb.AuthnRule) error {
	if r == nil {
		return nil
	}
	switch r.Mode {
	case authnpb.AuthnRule_MODE_UNSPECIFIED:
		if len(r.Schemes) > 0 {
			return fmt.Errorf(
				"%s: service %s method %s: invalid AuthnRule with MODE_UNSPECIFIED and non-empty schemes %v — set mode explicitly or remove schemes",
				file, service, method, r.Schemes,
			)
		}
	case authnpb.AuthnRule_MODE_PUBLIC:
		if len(r.Schemes) > 0 {
			return fmt.Errorf(
				"%s: service %s method %s: invalid AuthnRule with MODE_PUBLIC and non-empty schemes %v — public methods must not declare schemes",
				file, service, method, r.Schemes,
			)
		}
	}
	return nil
}

// writeFile emits authn_rules.gen.go containing a single aggregate
// AuthnRules() func that returns a map keyed by operation and valued by the
// authn annotation proto type.
func writeFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, rules []ruleEntry) {
	authnPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/api/gen/go/servora/authn/v1")
	protoPkg := protogen.GoImportPath("google.golang.org/protobuf/proto")

	g.P("// Code generated by protoc-gen-servora-authn. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	ruleIdent := g.QualifiedGoIdent(protogen.GoIdent{GoName: "AuthnRule", GoImportPath: authnPkg})
	protoClone := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Clone", GoImportPath: protoPkg})

	g.P("// _authnRules is the immutable backing store for AuthnRules.")
	g.P("var _authnRules = map[string]*", ruleIdent, "{")
	for _, r := range rules {
		modeIdent := g.QualifiedGoIdent(protogen.GoIdent{
			GoName:       "AuthnRule_" + r.Mode.String(),
			GoImportPath: authnPkg,
		})
		g.P(fmt.Sprintf("\t%q: {", r.Operation))
		g.P("\t\tMode: ", modeIdent, ",")
		if len(r.Schemes) > 0 {
			g.P("\t\tSchemes: []string{")
			for _, s := range r.Schemes {
				g.P(fmt.Sprintf("\t\t\t%q,", s))
			}
			g.P("\t\t},")
		}
		g.P("\t},")
	}
	g.P("}")
	g.P()

	g.P("// AuthnRules returns the authentication rules declared via authn proto")
	g.P("// annotations. Each call returns a fresh map and cloned rule messages.")
	g.P("func AuthnRules() map[string]*", ruleIdent, " {")
	g.P("\tm := make(map[string]*", ruleIdent, ", len(_authnRules))")
	g.P("\tfor k, v := range _authnRules {")
	g.P("\t\tm[k] = ", protoClone, "(v).(*", ruleIdent, ")")
	g.P("\t}")
	g.P("\treturn m")
	g.P("}")
}

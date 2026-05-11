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
//   - mode == MODE_PUBLIC with non-empty schemes (mutually exclusive),
//   - mode == MODE_REQUIRED with empty schemes (must specify schemes).
package main

import (
	"fmt"
	"path"
	"sort"
	"strings"

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
	type dirGroup struct {
		targetFile *protogen.File
		seen       map[string]bool
		public     []string
		schemes    map[string][]string
	}

	groups := map[string]*dirGroup{}

	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		for _, svc := range f.Services {
			svcDefault := serviceDefault(svc)
			for _, m := range svc.Methods {
				rule, hasMethod := methodRule(m)

				// Validate method-level rule, if present, before merging.
				if hasMethod {
					if err := validateRule(f.Desc.Path(), string(svc.Desc.Name()), string(m.Desc.Name()), rule); err != nil {
						return err
					}
				}

				merged, ok := optionmerge.Merge(svcDefault, rule, hasMethod)
				if !ok {
					continue
				}

				// Validate the merged rule too — service-level default may be
				// invalid on its own (e.g. REQUIRED with empty schemes), and a
				// method that inherits such a default should fail just as
				// loudly as if the rule had been written on the method itself.
				if err := validateRule(f.Desc.Path(), string(svc.Desc.Name()), string(m.Desc.Name()), merged); err != nil {
					return err
				}

				dir := path.Dir(f.GeneratedFilenamePrefix)
				if groups[dir] == nil {
					groups[dir] = &dirGroup{
						targetFile: f,
						seen:       map[string]bool{},
						schemes:    map[string][]string{},
					}
				}
				op := fmt.Sprintf("/%s/%s", svc.Desc.FullName(), m.Desc.Name())
				if groups[dir].seen[op] {
					continue
				}
				groups[dir].seen[op] = true

				switch merged.Mode {
				case authnpb.AuthnRule_MODE_PUBLIC:
					groups[dir].public = append(groups[dir].public, op)
				case authnpb.AuthnRule_MODE_REQUIRED:
					// Defensive copy so the generated file owns its slice.
					schemesCopy := append([]string(nil), merged.Schemes...)
					groups[dir].schemes[op] = schemesCopy
				}
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
		sort.Strings(g.public)
		schemeKeys := make([]string, 0, len(g.schemes))
		for k := range g.schemes {
			schemeKeys = append(schemeKeys, k)
		}
		sort.Strings(schemeKeys)

		gf := gen.NewGeneratedFile(
			path.Join(dir, "authn_rules.gen.go"),
			g.targetFile.GoImportPath,
		)
		writeFile(gf, g.targetFile.GoPackageName, g.public, g.schemes, schemeKeys)
	}

	return nil
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
	case authnpb.AuthnRule_MODE_REQUIRED:
		if len(r.Schemes) == 0 {
			return fmt.Errorf(
				"%s: service %s method %s: invalid AuthnRule with MODE_REQUIRED and empty schemes — required methods must list at least one scheme",
				file, service, method,
			)
		}
	}
	return nil
}

// writeFile emits authn_rules.gen.go containing a single aggregate
// AuthnRules() func that returns an authn.Rules value. Each call allocates
// fresh slices and a fresh map (with deep-copied inner slices) so callers
// cannot mutate the package-internal state.
func writeFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, public []string, schemes map[string][]string, schemeKeys []string) {
	authnPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/security/authn")

	g.P("// Code generated by protoc-gen-servora-authn. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	rulesIdent := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Rules", GoImportPath: authnPkg})

	// _authnRules backing struct literal.
	g.P("// _authnRules is the immutable backing store for AuthnRules.")
	g.P("var _authnRules = ", rulesIdent, "{")

	g.P("\tPublicMethods: []string{")
	for _, op := range public {
		g.P(fmt.Sprintf("\t\t%q,", op))
	}
	g.P("\t},")

	g.P("\tMethodSchemes: map[string][]string{")
	for _, k := range schemeKeys {
		v := schemes[k]
		var b strings.Builder
		for i, s := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", s)
		}
		g.P(fmt.Sprintf("\t\t%q: {%s},", k, b.String()))
	}
	g.P("\t},")
	g.P("}")
	g.P()

	// AuthnRules accessor — deep copy (PublicMethods slice + MethodSchemes
	// map + inner schemes slices). Each call returns an independent value so
	// callers may freely mutate the result.
	g.P("// AuthnRules returns the authentication rules declared via authn proto")
	g.P("// annotations. Each call allocates fresh slices and a fresh map (with")
	g.P("// deep-copied inner slices); callers may mutate the returned value freely")
	g.P("// without affecting other callers or package-internal state.")
	g.P("func AuthnRules() ", rulesIdent, " {")
	g.P("\tpm := make([]string, len(_authnRules.PublicMethods))")
	g.P("\tcopy(pm, _authnRules.PublicMethods)")
	g.P("\tms := make(map[string][]string, len(_authnRules.MethodSchemes))")
	g.P("\tfor k, v := range _authnRules.MethodSchemes {")
	g.P("\t\tcp := make([]string, len(v))")
	g.P("\t\tcopy(cp, v)")
	g.P("\t\tms[k] = cp")
	g.P("\t}")
	g.P("\treturn ", rulesIdent, "{PublicMethods: pm, MethodSchemes: ms}")
	g.P("}")
}

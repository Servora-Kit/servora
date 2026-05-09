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

				merged, ok := mergeRules(svcDefault, rule, hasMethod)
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

// mergeRules implements the spec's merge semantics. Returns (rule, true) when
// the method should appear in generated output; (_, false) when neither
// service nor method declared anything.
//
// Method-level rule with mode != UNSPECIFIED *fully replaces* the service
// default — including its schemes. UNSPECIFIED (or absence) inherits the
// service default verbatim.
func mergeRules(svcDefault *authnpb.AuthnRule, methodRule *authnpb.AuthnRule, hasMethod bool) (*authnpb.AuthnRule, bool) {
	switch {
	case hasMethod && methodRule.Mode != authnpb.AuthnRule_MODE_UNSPECIFIED:
		// Method-level wins outright.
		return &authnpb.AuthnRule{
			Mode:    methodRule.Mode,
			Schemes: append([]string(nil), methodRule.Schemes...),
		}, true
	case svcDefault != nil && svcDefault.Mode != authnpb.AuthnRule_MODE_UNSPECIFIED:
		// Inherit service default.
		return &authnpb.AuthnRule{
			Mode:    svcDefault.Mode,
			Schemes: append([]string(nil), svcDefault.Schemes...),
		}, true
	}
	// Neither side contributes a usable rule.
	return nil, false
}

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

// writeFile emits authn_rules.gen.go. The accessors (PublicMethods,
// MethodSchemes) always allocate fresh slices/maps so callers cannot mutate
// the package-internal state.
func writeFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, public []string, schemes map[string][]string, schemeKeys []string) {
	g.P("// Code generated by protoc-gen-servora-authn. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	// _publicMethods backing slice.
	g.P("// _publicMethods is the immutable backing slice of public RPC paths.")
	g.P("var _publicMethods = []string{")
	for _, op := range public {
		g.P(fmt.Sprintf("\t%q,", op))
	}
	g.P("}")
	g.P()

	// PublicMethods accessor — allocates a fresh slice and copies in.
	g.P("// PublicMethods returns a copy of the public RPC paths declared via")
	g.P("// authn proto annotations. Each call allocates a new slice; callers may")
	g.P("// mutate the returned value freely without affecting other callers.")
	g.P("func PublicMethods() []string {")
	g.P("\tout := make([]string, len(_publicMethods))")
	g.P("\tcopy(out, _publicMethods)")
	g.P("\treturn out")
	g.P("}")
	g.P()

	// _methodSchemes backing map.
	g.P("// _methodSchemes is the immutable backing map of REQUIRED RPC paths to")
	g.P("// their accepted authentication schemes.")
	g.P("var _methodSchemes = map[string][]string{")
	for _, k := range schemeKeys {
		v := schemes[k]
		var b strings.Builder
		for i, s := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", s)
		}
		g.P(fmt.Sprintf("\t%q: {%s},", k, b.String()))
	}
	g.P("}")
	g.P()

	// MethodSchemes accessor — deep copy (map + slices).
	g.P("// MethodSchemes returns a copy of the RPC-path → schemes table for")
	g.P("// REQUIRED methods. Each call allocates a fresh map and per-key slice;")
	g.P("// callers may mutate the result without affecting other callers.")
	g.P("func MethodSchemes() map[string][]string {")
	g.P("\tout := make(map[string][]string, len(_methodSchemes))")
	g.P("\tfor k, v := range _methodSchemes {")
	g.P("\t\tcp := make([]string, len(v))")
	g.P("\t\tcopy(cp, v)")
	g.P("\t\tout[k] = cp")
	g.P("\t}")
	g.P("\treturn out")
	g.P("}")
}

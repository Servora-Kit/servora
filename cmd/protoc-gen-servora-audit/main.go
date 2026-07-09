// Command protoc-gen-servora-audit translates servora audit proto annotations
// into a Go file (`audit_rules.gen.go`) consumed by the audit middleware at runtime.
//
// Merge semantics (matches authn / authz):
//   - method-level rule with mode != AUDIT_MODE_UNSPECIFIED replaces the
//     service-level default in its entirety,
//   - method-level rule absent (or mode == AUDIT_MODE_UNSPECIFIED) inherits
//     the service-level default,
//   - only methods whose merged mode is AUDIT_MODE_ENABLED reach the generated
//     output; AUDIT_MODE_DISABLED and methods with no resolved rule are skipped.
//
// Cross-file template scanning: rules are gathered from ALL input files
// (including non-generated dependencies). Templates are keyed by proto
// fully-qualified service name so same-named services in different packages do
// not share rules. Generated output groups by output directory so each
// directory yields one audit_rules.gen.go covering the services declared in it.
package main

import (
	"fmt"
	"path"
	"sort"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
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

// ruleEntry carries the information needed to generate one AuditRule map entry.
type ruleEntry struct {
	Operation  string
	MethodName string
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
	serviceDefaults := map[string]*auditv1.AuditRule{}
	methodTemplates := map[string]map[string]*auditv1.AuditRule{}
	for _, f := range gen.Files {
		for _, svc := range f.Services {
			fullName := string(svc.Desc.FullName())

			if def := serviceDefault(svc); def != nil {
				serviceDefaults[fullName] = def
			}

			for _, m := range svc.Methods {
				if r, ok := methodRule(m); ok {
					if methodTemplates[fullName] == nil {
						methodTemplates[fullName] = map[string]*auditv1.AuditRule{}
					}
					methodTemplates[fullName][string(m.Desc.Name())] = r
				}
			}
		}
	}

	// Second pass: emit one audit_rules.gen.go per directory that has at
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

			for _, m := range svc.Methods {
				methodName := string(m.Desc.Name())
				methodR := methods[methodName]
				merged, ok := optionmerge.Merge(svcDefault, methodR, methodR != nil)
				if !ok {
					continue
				}
				if merged.Mode != auditv1.AuditMode_AUDIT_MODE_ENABLED {
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
					Operation:  op,
					MethodName: methodName,
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
			path.Join(dir, "audit_rules.gen.go"),
			group.targetFile.GoImportPath,
		)
		generateFile(g, group.targetFile.GoPackageName, group.rules)
	}

	return nil
}

// methodRule extracts the AuditRule attached to a method via E_AuditRule,
// returning hasMethod == true only when the extension is present.
func methodRule(m *protogen.Method) (*auditv1.AuditRule, bool) {
	opts := m.Desc.Options()
	if opts == nil {
		return nil, false
	}
	if !proto.HasExtension(opts, auditv1.E_AuditRule) {
		return nil, false
	}
	r, ok := proto.GetExtension(opts, auditv1.E_AuditRule).(*auditv1.AuditRule)
	if !ok || r == nil {
		return nil, false
	}
	return r, true
}

// serviceDefault extracts the service-level default (E_ServiceDefault), or
// nil when none is declared.
func serviceDefault(s *protogen.Service) *auditv1.AuditRule {
	opts := s.Desc.Options()
	if opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, auditv1.E_ServiceDefault) {
		return nil
	}
	r, ok := proto.GetExtension(opts, auditv1.E_ServiceDefault).(*auditv1.AuditRule)
	if !ok || r == nil {
		return nil
	}
	return r
}

func generateFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, rules []ruleEntry) {
	auditv1Pkg := protogen.GoImportPath("github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1")

	g.P("// Code generated by protoc-gen-servora-audit. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	auditRule := g.QualifiedGoIdent(protogen.GoIdent{GoName: "AuditRule", GoImportPath: auditv1Pkg})
	auditModeEnabled := g.QualifiedGoIdent(protogen.GoIdent{GoName: "AuditMode_AUDIT_MODE_ENABLED", GoImportPath: auditv1Pkg})

	g.P("// AuditRules returns the audit rules for this package.")
	g.P("// Merge these with other packages' rules using audit.Middleware WithRulesFuncs.")
	g.P("func AuditRules() map[string]*", auditRule, " {")
	g.P("	return map[string]*", auditRule, "{")
	for _, r := range rules {
		g.P(fmt.Sprintf("		%q: {Mode: ", r.Operation), auditModeEnabled, "},")
	}
	g.P("	}")
	g.P("}")
}

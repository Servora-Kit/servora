// Command protoc-gen-servora-audit translates servora audit proto annotations
// into a Go file (`audit_rules.gen.go`) that the runtime consults to decide
// which RPC methods produce audit events.
//
// Merge semantics (matches authn / authz):
//   - method-level rule with mode != AUDIT_MODE_UNSPECIFIED replaces the
//     service-level default in its entirety,
//   - method-level rule absent (or mode == AUDIT_MODE_UNSPECIFIED) inherits
//     the service-level default,
//   - only methods whose merged mode is AUDIT_MODE_ENABLED reach the generated
//     output; AUDIT_MODE_DISABLED and methods with no resolved rule are
//     skipped.
//
// Schema fold: AuditRule.RecordOnError is an ErrorRecordMode enum on the wire,
// but the runtime audit.Rule.RecordOnError field stays a bool. The plugin
// folds ERROR_RECORD_MODE_ON → true and every other value (OFF /
// UNSPECIFIED) → false at codegen time so callers see a stable Go API.
package main

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
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

type ruleEntry struct {
	Operation     string
	EventType     auditv1.AuditEventType
	MutationType  auditv1.ResourceMutationType
	TargetType    string
	TargetIDPath  string // e.g. "resp.id", "req.id"
	RecordOnError bool   // folded from ErrorRecordMode enum at codegen time
	MethodName    string // for TargetIDFunc naming
}

// generate is the testable entry point. It scans every file in gen.Files,
// computes the merged audit rules, and emits one audit_rules.gen.go per output
// directory that has at least one ENABLED rule.
func generate(gen *protogen.Plugin) error {
	// Build service alias index: shortName → []fullName across all files. This
	// mirrors the authz plugin so operation paths cover every full-package
	// variant (matters when an HTTP-gateway proto re-declares the same
	// service name as its pure-RPC counterpart).
	svcAliases := map[string][]string{}
	for _, f := range gen.Files {
		for _, svc := range f.Services {
			name := string(svc.Desc.Name())
			full := string(svc.Desc.FullName())
			svcAliases[name] = append(svcAliases[name], full)
		}
	}

	// Group rules by output directory; each directory yields one
	// audit_rules.gen.go.
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
		for _, svc := range f.Services {
			svcDefault := serviceDefault(svc)
			for _, m := range svc.Methods {
				rule, hasMethod := methodRule(m)
				merged, ok := mergeRules(svcDefault, rule, hasMethod)
				if !ok {
					continue
				}
				if merged.Mode != auditv1.AuditMode_AUDIT_MODE_ENABLED {
					// DISABLED (explicit or merged) drops the method.
					continue
				}

				methodName := string(m.Desc.Name())
				svcName := string(svc.Desc.Name())
				dir := path.Dir(f.GeneratedFilenamePrefix)
				if groups[dir] == nil {
					groups[dir] = &dirGroup{
						targetFile: f,
						seen:       map[string]bool{},
					}
				}
				for _, fullSvc := range svcAliases[svcName] {
					op := fmt.Sprintf("/%s/%s", fullSvc, methodName)
					if groups[dir].seen[op] {
						continue
					}
					groups[dir].seen[op] = true
					groups[dir].rules = append(groups[dir].rules, ruleEntry{
						Operation:     op,
						EventType:     merged.EventType,
						MutationType:  merged.MutationType,
						TargetType:    merged.TargetType,
						TargetIDPath:  merged.TargetIdField,
						RecordOnError: merged.RecordOnError == auditv1.ErrorRecordMode_ERROR_RECORD_MODE_ON,
						MethodName:    methodName,
					})
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
		sort.Slice(g.rules, func(i, j int) bool {
			return g.rules[i].Operation < g.rules[j].Operation
		})
		gf := gen.NewGeneratedFile(
			path.Join(dir, "audit_rules.gen.go"),
			g.targetFile.GoImportPath,
		)
		generateFile(gf, g.targetFile.GoPackageName, g.rules)
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

// mergeRules implements the spec's merge semantics. Returns (rule, true) when
// the method should be considered for emission; (_, false) when neither side
// declared anything.
//
// A method-level rule with mode != AUDIT_MODE_UNSPECIFIED fully replaces the
// service default. UNSPECIFIED (or absence) inherits the service default
// verbatim. The caller then filters by `mode == ENABLED` for emission.
func mergeRules(svcDefault *auditv1.AuditRule, methodRule *auditv1.AuditRule, hasMethod bool) (*auditv1.AuditRule, bool) {
	switch {
	case hasMethod && methodRule.Mode != auditv1.AuditMode_AUDIT_MODE_UNSPECIFIED:
		// Method-level wins outright. Copy fields so callers cannot mutate
		// the source descriptor.
		return cloneAuditRule(methodRule), true
	case svcDefault != nil && svcDefault.Mode != auditv1.AuditMode_AUDIT_MODE_UNSPECIFIED:
		return cloneAuditRule(svcDefault), true
	}
	return nil, false
}

func cloneAuditRule(r *auditv1.AuditRule) *auditv1.AuditRule {
	return &auditv1.AuditRule{
		Mode:          r.Mode,
		EventType:     r.EventType,
		MutationType:  r.MutationType,
		TargetType:    r.TargetType,
		TargetIdField: r.TargetIdField,
		RecordOnError: r.RecordOnError,
	}
}

func generateFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, rules []ruleEntry) {
	auditPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/obs/audit")
	auditv1Pkg := protogen.GoImportPath("github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1")

	g.P("// Code generated by protoc-gen-servora-audit. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	auditRule := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Rule", GoImportPath: auditPkg})

	// Emit target ID extractor functions for rules with target_id_field set.
	for _, r := range rules {
		if r.TargetIDPath == "" {
			continue
		}
		funcName := extractFuncName(r.MethodName)
		prefix, field := parseTargetIDPath(r.TargetIDPath)
		goField := goGetter(field)

		g.P(fmt.Sprintf("func %s(req, resp any) string {", funcName))
		if prefix == "resp" {
			g.P(fmt.Sprintf("	if r, ok := resp.(interface{ Get%s() string }); ok && r != nil {", goField))
			g.P(fmt.Sprintf("		return r.Get%s()", goField))
		} else {
			g.P(fmt.Sprintf("	if r, ok := req.(interface{ Get%s() string }); ok && r != nil {", goField))
			g.P(fmt.Sprintf("		return r.Get%s()", goField))
		}
		g.P("	}")
		g.P("	return \"\"")
		g.P("}")
		g.P()
	}

	// Emit backing var.
	g.P("// _auditRules is the immutable backing store for AuditRules.")
	g.P("var _auditRules = map[string]", auditRule, "{")
	for _, r := range rules {
		g.P(fmt.Sprintf("	%q: {", r.Operation))

		// EventType
		if r.EventType != auditv1.AuditEventType_AUDIT_EVENT_TYPE_UNSPECIFIED {
			evtIdent := g.QualifiedGoIdent(protogen.GoIdent{
				GoName:       "EventType" + auditEventTypeSuffix(r.EventType),
				GoImportPath: auditPkg,
			})
			g.P("		EventType: ", evtIdent, ",")
		}

		// MutationType
		if r.MutationType != auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_UNSPECIFIED {
			mutIdent := g.QualifiedGoIdent(protogen.GoIdent{
				GoName:       "ResourceMutation" + mutationTypeSuffix(r.MutationType),
				GoImportPath: auditPkg,
			})
			g.P("		MutationType: ", mutIdent, ",")
		}

		if r.TargetType != "" {
			g.P(fmt.Sprintf("		TargetType: %q,", r.TargetType))
		}

		if r.RecordOnError {
			g.P("		RecordOnError: true,")
		}

		if r.TargetIDPath != "" {
			funcName := extractFuncName(r.MethodName)
			g.P(fmt.Sprintf("		TargetIDFunc: %s,", funcName))
		}

		g.P("	},")
	}
	g.P("}")
	g.P()

	// Emit copy-returning func.
	_ = auditv1Pkg // imported for proto enum resolution context
	g.P("// AuditRules returns a copy of the audit rules for this package.")
	g.P("// Each call returns an independent map; callers may not modify the returned map.")
	g.P("func AuditRules() map[string]", auditRule, " {")
	g.P("	m := make(map[string]", auditRule, ", len(_auditRules))")
	g.P("	for k, v := range _auditRules {")
	g.P("		m[k] = v")
	g.P("	}")
	g.P("	return m")
	g.P("}")
}

// extractFuncName returns the TargetIDFunc name for a given method name.
func extractFuncName(methodName string) string {
	return fmt.Sprintf("_extract%sTargetID", methodName)
}

// parseTargetIDPath splits "req.field.sub" into ("req", "field.sub").
func parseTargetIDPath(p string) (prefix, rest string) {
	parts := strings.SplitN(p, ".", 2)
	if len(parts) != 2 {
		return "req", p
	}
	return parts[0], parts[1]
}

// goGetter converts a snake_case field path to a PascalCase Go getter name.
// E.g. "user_id" → "UserId", "id" → "Id".
func goGetter(field string) string {
	parts := strings.Split(field, "_")
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

// auditEventTypeSuffix maps proto enum to Go audit.EventType* suffix.
func auditEventTypeSuffix(t auditv1.AuditEventType) string {
	switch t {
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_RESOURCE_MUTATION:
		return "ResourceMutation"
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHZ_DECISION:
		return "AuthzDecision"
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_AUTHN_RESULT:
		return "AuthnResult"
	case auditv1.AuditEventType_AUDIT_EVENT_TYPE_TUPLE_CHANGED:
		return "TupleChanged"
	default:
		return "ResourceMutation"
	}
}

// mutationTypeSuffix maps proto enum to Go audit.ResourceMutation* suffix.
func mutationTypeSuffix(t auditv1.ResourceMutationType) string {
	switch t {
	case auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_CREATE:
		return "Create"
	case auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_UPDATE:
		return "Update"
	case auditv1.ResourceMutationType_RESOURCE_MUTATION_TYPE_DELETE:
		return "Delete"
	default:
		return "Update"
	}
}

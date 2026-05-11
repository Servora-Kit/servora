// Command protoc-gen-servora-audit translates servora audit proto annotations
// into a Go file (`audit_rules.gen.go`) that exports a map of
// audit.CompiledRule entries consumed by the audit middleware at runtime.
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
// Generated output per proto package:
//
//	func AuditRules() map[string]*audit.CompiledRule { ... }
//
// Each CompiledRule includes Mode, EventType, Severity and a BuildEvent func
// that constructs a CloudEvents event from request/response payloads.
package main

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	auditv1 "github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1"
	cev1 "github.com/Servora-Kit/servora/api/gen/go/servora/cloudevents/v1"
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

// extensionEntry represents a single CloudEvents extension mapping.
type extensionEntry struct {
	Name      string
	FromField string                                   // set when source is from_field
	Literal   *cev1.CloudEvent_CloudEventAttributeValue // set when source is literal
}

// ruleEntry carries all information needed to generate one CompiledRule map entry.
type ruleEntry struct {
	Operation          string
	EventType          string
	Severity           string
	DetailMessageField string // e.g. "req", "resp", "req.user"
	TargetIDField      string // e.g. "resp.id", "req.id"
	Extensions         []extensionEntry
	MethodName         string // for naming and comments

	// GoIdents for request/response types (used for type assertions in BuildEvent).
	InputIdent  protogen.GoIdent
	OutputIdent protogen.GoIdent
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
				merged, ok := optionmerge.Merge(svcDefault, rule, hasMethod)
				if !ok {
					continue
				}
				if merged.Mode != auditv1.AuditMode_AUDIT_MODE_ENABLED {
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

				// Collect extensions.
				var exts []extensionEntry
				for _, ext := range merged.Extensions {
					entry := extensionEntry{Name: ext.GetName()}
					switch s := ext.GetSource().(type) {
					case *auditv1.ExtensionMapping_FromField:
						entry.FromField = s.FromField
					case *auditv1.ExtensionMapping_Literal:
						entry.Literal = s.Literal
					}
					exts = append(exts, entry)
				}

				for _, fullSvc := range svcAliases[svcName] {
					op := fmt.Sprintf("/%s/%s", fullSvc, methodName)
					if groups[dir].seen[op] {
						continue
					}
					groups[dir].seen[op] = true
					groups[dir].rules = append(groups[dir].rules, ruleEntry{
						Operation:          op,
						EventType:          merged.EventType,
						Severity:           merged.Severity,
						DetailMessageField: merged.DetailMessageField,
						TargetIDField:      merged.TargetIdField,
						Extensions:         exts,
						MethodName:         methodName,
						InputIdent:         m.Input.GoIdent,
						OutputIdent:        m.Output.GoIdent,
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

// mergeRules is now provided by cmd/internal/optionmerge.Merge.

func generateFile(g *protogen.GeneratedFile, pkgName protogen.GoPackageName, rules []ruleEntry) {
	auditPkg := protogen.GoImportPath("github.com/Servora-Kit/servora/obs/audit")
	cePkg := protogen.GoImportPath("github.com/cloudevents/sdk-go/v2")
	auditv1Pkg := protogen.GoImportPath("github.com/Servora-Kit/servora/api/gen/go/servora/audit/v1")

	g.P("// Code generated by protoc-gen-servora-audit. DO NOT EDIT.")
	g.P()
	g.P("package ", pkgName)
	g.P()

	// Force imports we always need.
	contextIdent := protogen.GoIdent{GoName: "Context", GoImportPath: "context"}
	_ = g.QualifiedGoIdent(contextIdent) // ensure "context" is imported

	compiledRule := g.QualifiedGoIdent(protogen.GoIdent{GoName: "CompiledRule", GoImportPath: auditPkg})
	ceEvent := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Event", GoImportPath: cePkg})
	auditModeEnabled := g.QualifiedGoIdent(protogen.GoIdent{GoName: "AuditMode_AUDIT_MODE_ENABLED", GoImportPath: auditv1Pkg})

	// Emit the AuditRules function.
	g.P("// AuditRules returns the compiled audit rules for this package.")
	g.P("// The middleware merges these with other packages' rules at startup.")
	g.P("func AuditRules() map[string]*", compiledRule, " {")
	g.P("	return map[string]*", compiledRule, "{")

	for _, r := range rules {
		g.P(fmt.Sprintf("		%q: {", r.Operation))
		g.P("			Mode: int32(", auditModeEnabled, "),")
		if r.EventType != "" {
			g.P(fmt.Sprintf("			EventType: %q,", r.EventType))
		}
		if r.Severity != "" {
			g.P(fmt.Sprintf("			Severity: %q,", r.Severity))
		}

		// BuildEvent closure.
		g.P("			BuildEvent: func(ctx ", g.QualifiedGoIdent(contextIdent), ", req, resp any, err error) ", ceEvent, " {")
		generateBuildEventBody(g, r, auditPkg)
		g.P("			},")
		g.P("		},")
	}

	g.P("	}")
	g.P("}")
}

// generateBuildEventBody emits the body of a BuildEvent closure for a single rule.
func generateBuildEventBody(g *protogen.GeneratedFile, r ruleEntry, auditPkg protogen.GoImportPath) {
	newEvent := g.QualifiedGoIdent(protogen.GoIdent{GoName: "NewEvent", GoImportPath: auditPkg})
	withType := g.QualifiedGoIdent(protogen.GoIdent{GoName: "WithType", GoImportPath: auditPkg})

	// Construct event with type.
	if r.EventType != "" {
		g.P(fmt.Sprintf("				e := %s(ctx, %s(%q))", newEvent, withType, r.EventType))
	} else {
		g.P(fmt.Sprintf("				e := %s(ctx)", newEvent))
	}

	// Severity.
	if r.Severity != "" {
		extSeverityText := g.QualifiedGoIdent(protogen.GoIdent{GoName: "ExtSeverityText", GoImportPath: auditPkg})
		g.P(fmt.Sprintf("				e.SetExtension(%s, %q)", extSeverityText, r.Severity))
	}

	// detail_message_field: serialize proto message as data payload.
	if r.DetailMessageField != "" {
		setProtoData := g.QualifiedGoIdent(protogen.GoIdent{GoName: "SetProtoData", GoImportPath: auditPkg})
		prefix, fieldPath := parseTargetIDPath(r.DetailMessageField)
		if fieldPath == "" || fieldPath == r.DetailMessageField {
			// Whole message (e.g. "req" or "resp").
			if prefix == "resp" {
				respType := g.QualifiedGoIdent(r.OutputIdent)
				g.P(fmt.Sprintf("				if r, ok := resp.(*%s); ok && r != nil {", respType))
				g.P(fmt.Sprintf("					_ = %s(&e, r)", setProtoData))
				g.P("				}")
			} else {
				reqType := g.QualifiedGoIdent(r.InputIdent)
				g.P(fmt.Sprintf("				if r, ok := req.(*%s); ok && r != nil {", reqType))
				g.P(fmt.Sprintf("					_ = %s(&e, r)", setProtoData))
				g.P("				}")
			}
		} else {
			// Nested field (e.g. "resp.user").
			getterChain := buildGetterChain(fieldPath)
			if prefix == "resp" {
				respType := g.QualifiedGoIdent(r.OutputIdent)
				g.P(fmt.Sprintf("				if r, ok := resp.(*%s); ok && r != nil {", respType))
				g.P(fmt.Sprintf("					if detail := r.%s; detail != nil {", getterChain))
				g.P(fmt.Sprintf("						_ = %s(&e, detail)", setProtoData))
				g.P("					}")
				g.P("				}")
			} else {
				reqType := g.QualifiedGoIdent(r.InputIdent)
				g.P(fmt.Sprintf("				if r, ok := req.(*%s); ok && r != nil {", reqType))
				g.P(fmt.Sprintf("					if detail := r.%s; detail != nil {", getterChain))
				g.P(fmt.Sprintf("						_ = %s(&e, detail)", setProtoData))
				g.P("					}")
				g.P("				}")
			}
		}
	}

	// target_id_field: extract ID and set as subject.
	if r.TargetIDField != "" {
		prefix, fieldPath := parseTargetIDPath(r.TargetIDField)
		getterChain := buildGetterChain(fieldPath)
		if prefix == "resp" {
			respType := g.QualifiedGoIdent(r.OutputIdent)
			g.P(fmt.Sprintf("				if r, ok := resp.(*%s); ok && r != nil {", respType))
			g.P(fmt.Sprintf("					e.SetSubject(r.%s)", getterChain))
			g.P("				}")
		} else {
			reqType := g.QualifiedGoIdent(r.InputIdent)
			g.P(fmt.Sprintf("				if r, ok := req.(*%s); ok && r != nil {", reqType))
			g.P(fmt.Sprintf("					e.SetSubject(r.%s)", getterChain))
			g.P("				}")
		}
	}

	// extensions: set each extension from field or literal.
	for _, ext := range r.Extensions {
		if ext.FromField != "" {
			prefix, fieldPath := parseTargetIDPath(ext.FromField)
			getterChain := buildGetterChain(fieldPath)
			if prefix == "resp" {
				respType := g.QualifiedGoIdent(r.OutputIdent)
				g.P(fmt.Sprintf("				if r, ok := resp.(*%s); ok && r != nil {", respType))
				g.P(fmt.Sprintf("					e.SetExtension(%q, r.%s)", ext.Name, getterChain))
				g.P("				}")
			} else {
				reqType := g.QualifiedGoIdent(r.InputIdent)
				g.P(fmt.Sprintf("				if r, ok := req.(*%s); ok && r != nil {", reqType))
				g.P(fmt.Sprintf("					e.SetExtension(%q, r.%s)", ext.Name, getterChain))
				g.P("				}")
			}
		} else if ext.Literal != nil {
			literalCode := literalValueCode(ext.Literal)
			g.P(fmt.Sprintf("				e.SetExtension(%q, %s)", ext.Name, literalCode))
		}
	}

	// Error handling: override severity and add error message.
	extSeverityText := g.QualifiedGoIdent(protogen.GoIdent{GoName: "ExtSeverityText", GoImportPath: auditPkg})
	extErrorMessage := g.QualifiedGoIdent(protogen.GoIdent{GoName: "ExtErrorMessage", GoImportPath: auditPkg})
	g.P("				if err != nil {")
	g.P(fmt.Sprintf("					e.SetExtension(%s, \"ERROR\")", extSeverityText))
	g.P(fmt.Sprintf("					e.SetExtension(%s, err.Error())", extErrorMessage))
	g.P("				}")
	g.P("				return e")
}

// buildGetterChain converts a field path like "user.id" into "GetUser().GetId()".
func buildGetterChain(fieldPath string) string {
	parts := strings.Split(fieldPath, ".")
	var b strings.Builder
	for _, part := range parts {
		b.WriteString("Get")
		b.WriteString(snakeToPascal(part))
		b.WriteString("()")
		// Chain with . for next getter.
		b.WriteString(".")
	}
	// Remove trailing ".".
	s := b.String()
	if len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}

// snakeToPascal converts a snake_case identifier to PascalCase.
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
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

// literalValueCode returns a Go expression string for a CloudEventAttributeValue literal.
func literalValueCode(v *cev1.CloudEvent_CloudEventAttributeValue) string {
	if v == nil {
		return `""`
	}
	switch attr := v.GetAttr().(type) {
	case *cev1.CloudEvent_CloudEventAttributeValue_CeString:
		return fmt.Sprintf("%q", attr.CeString)
	case *cev1.CloudEvent_CloudEventAttributeValue_CeInteger:
		return fmt.Sprintf("int32(%d)", attr.CeInteger)
	case *cev1.CloudEvent_CloudEventAttributeValue_CeBoolean:
		if attr.CeBoolean {
			return "true"
		}
		return "false"
	case *cev1.CloudEvent_CloudEventAttributeValue_CeBytes:
		return fmt.Sprintf("[]byte(%q)", string(attr.CeBytes))
	case *cev1.CloudEvent_CloudEventAttributeValue_CeUri:
		return fmt.Sprintf("%q", attr.CeUri)
	case *cev1.CloudEvent_CloudEventAttributeValue_CeUriRef:
		return fmt.Sprintf("%q", attr.CeUriRef)
	case *cev1.CloudEvent_CloudEventAttributeValue_CeTimestamp:
		// Generate time.Unix(seconds, nanos) call.
		ts := attr.CeTimestamp
		if ts == nil {
			return `time.Time{}`
		}
		return fmt.Sprintf("time.Unix(%d, %d)", ts.GetSeconds(), ts.GetNanos())
	default:
		return `""`
	}
}

// parseTargetIDPath splits "req.field.sub" into ("req", "field.sub").
// If no dot is present, returns ("req", original).
func parseTargetIDPath(p string) (prefix, rest string) {
	parts := strings.SplitN(p, ".", 2)
	if len(parts) != 2 {
		return "req", p
	}
	return parts[0], parts[1]
}

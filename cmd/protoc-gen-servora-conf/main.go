// Command protoc-gen-servora-conf consumes servora.conf.v1 annotations on
// configuration messages and emits a companion <file>.pb.servora-conf.go that
// declares receiver methods bound to the generated *.pb.go types:
//
//   - SectionKey() string         // present when message has (section) annotation
//   - SectionOptional() bool      // present when section { optional: true }
//   - ApplyDefaults()             // present when a field has (field) { default: ... },
//                                 //   or the message transitively reaches one via a
//                                 //   singular message-typed field (cascade container);
//                                 //   oneof members use default-if-set (no allocation)
//   - CheckRequired() error       // present when any field has (field) { required: true },
//                                 //   or the message transitively reaches one (cascade);
//                                 //   oneof members are checked if set
//   - ApplyConf() error           // composite: CheckRequired → ApplyDefaults in canonical
//                                 //   order; runtime calls this single method via ConfApplier
//
// Output file is co-located with the source proto's *.pb.go and shares its
// Go package. Messages without any conf annotations are skipped so files with
// no relevant messages produce no output.
package main

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/cmd/internal/protoreach"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/pluginpb"
)

const generatedFileSuffix = ".pb.servora-conf.go"

func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return generate(gen)
	})
}

// generate is the testable entry point. For every input file that contains at
// least one annotated message it emits a single companion .pb.servora-conf.go.
func generate(gen *protogen.Plugin) error {
	for _, f := range gen.Files {
		if !f.Generate {
			continue
		}
		msgs := collectAnnotatedMessages(f.Messages)
		if len(msgs) == 0 {
			continue
		}
		// Deterministic ordering by full proto name.
		sort.Slice(msgs, func(i, j int) bool {
			return string(msgs[i].Desc.FullName()) < string(msgs[j].Desc.FullName())
		})

		outName := f.GeneratedFilenamePrefix + generatedFileSuffix
		gf := gen.NewGeneratedFile(outName, f.GoImportPath)
		emitHeader(gf, f, msgs[0].GoIdent.GoImportPath != "")
		gf.P("package ", f.GoPackageName)
		gf.P()

		for _, m := range msgs {
			if err := emitForMessage(gf, m); err != nil {
				return fmt.Errorf("emit %s: %w", m.Desc.FullName(), err)
			}
		}
	}
	return nil
}

// collectAnnotatedMessages flattens nested messages and keeps those with at
// least one servora.conf.v1 annotation (section or any field-level rule), or
// that transitively need a cascade method (ApplyDefaults or CheckRequired).
func collectAnnotatedMessages(in []*protogen.Message) []*protogen.Message {
	var out []*protogen.Message
	for _, m := range in {
		if messageHasAnnotations(m) ||
			protoreach.NeedsCascade(m.Desc, fieldHasDefault, map[protoreflect.FullName]bool{}) ||
			protoreach.NeedsCascade(m.Desc, fieldHasRequired, map[protoreflect.FullName]bool{}) {
			out = append(out, m)
		}
		out = append(out, collectAnnotatedMessages(m.Messages)...)
	}
	return out
}

func messageHasAnnotations(m *protogen.Message) bool {
	if sectionRule(m) != nil {
		return true
	}
	for _, f := range m.Fields {
		if fieldRule(f) != nil {
			return true
		}
	}
	return false
}

// fieldHasDefault is the leaf predicate for ApplyDefaults cascade: true when
// the field carries a non-empty (servora.conf.v1.field) { default: ... }.
func fieldHasDefault(fd protoreflect.FieldDescriptor) bool {
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Field) {
		return false
	}
	r, _ := proto.GetExtension(opts, confv1.E_Field).(*confv1.FieldRule)
	return r != nil && r.GetDefault() != ""
}

// fieldHasRequired is the leaf predicate for CheckRequired cascade: true when
// the field carries (servora.conf.v1.field) { required: true }.
func fieldHasRequired(fd protoreflect.FieldDescriptor) bool {
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Field) {
		return false
	}
	r, _ := proto.GetExtension(opts, confv1.E_Field).(*confv1.FieldRule)
	return r != nil && r.GetRequired()
}

func sectionRule(m *protogen.Message) *confv1.SectionRule {
	opts := m.Desc.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Section) {
		return nil
	}
	r, _ := proto.GetExtension(opts, confv1.E_Section).(*confv1.SectionRule)
	if r == nil || r.GetKey() == "" {
		return nil
	}
	return r
}

func fieldRule(f *protogen.Field) *confv1.FieldRule {
	opts := f.Desc.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Field) {
		return nil
	}
	r, _ := proto.GetExtension(opts, confv1.E_Field).(*confv1.FieldRule)
	if r == nil {
		return nil
	}
	if r.GetDefault() == "" && !r.GetRequired() {
		return nil
	}
	return r
}

func emitHeader(g *protogen.GeneratedFile, f *protogen.File, _ bool) {
	g.P("// Code generated by protoc-gen-servora-conf. DO NOT EDIT.")
	g.P("// source: ", f.Desc.Path())
	g.P()
}

func emitForMessage(g *protogen.GeneratedFile, m *protogen.Message) error {
	goType := m.GoIdent.GoName

	if sr := sectionRule(m); sr != nil {
		g.P("// SectionKey returns the configuration section key declared on ", goType, ".")
		g.P("func (*", goType, ") SectionKey() string { return ", strconv.Quote(sr.GetKey()), " }")
		g.P()
		if sr.GetOptional() {
			g.P("// SectionOptional reports whether the section may be absent from the config source.")
			g.P("func (*", goType, ") SectionOptional() bool { return true }")
			g.P()
		}
	}

	defaultsFields := collectDefaultFields(m)
	defRegular, defOneof := cascadeChildren(m, fieldHasDefault)
	hasDefaults := len(defaultsFields) > 0 || len(defRegular) > 0 || len(defOneof) > 0
	if hasDefaults {
		if err := emitApplyDefaults(g, m, defaultsFields, defRegular, defOneof); err != nil {
			return err
		}
	}

	requiredFields := collectRequiredFields(m)
	reqRegular, reqOneof := cascadeChildren(m, fieldHasRequired)
	hasRequired := len(requiredFields) > 0 || len(reqRegular) > 0 || len(reqOneof) > 0
	if hasRequired {
		emitCheckRequired(g, m, requiredFields, reqRegular, reqOneof)
	}

	if hasDefaults || hasRequired {
		emitApplyConf(g, m, hasRequired, hasDefaults)
	}
	return nil
}

type defaultEntry struct {
	field *protogen.Field
	rule  *confv1.FieldRule
}

func collectDefaultFields(m *protogen.Message) []defaultEntry {
	var out []defaultEntry
	for _, f := range m.Fields {
		r := fieldRule(f)
		if r == nil || r.GetDefault() == "" {
			continue
		}
		out = append(out, defaultEntry{field: f, rule: r})
	}
	return out
}

func collectRequiredFields(m *protogen.Message) []*protogen.Field {
	var out []*protogen.Field
	for _, f := range m.Fields {
		r := fieldRule(f)
		if r == nil || !r.GetRequired() {
			continue
		}
		out = append(out, f)
	}
	return out
}

// cascadeChildren returns singular message-typed fields whose message
// transitively satisfies pred. Fields are split into regular (non-oneof)
// and oneof groups — emitters use different patterns for each:
//   - regular: allocate-then-recurse (ApplyDefaults) or check-if-non-nil (CheckRequired)
//   - oneof:   recurse-if-set only (never allocate, would violate at-most-one-case)
func cascadeChildren(m *protogen.Message, pred func(protoreflect.FieldDescriptor) bool) (regular, oneof []*protogen.Field) {
	for _, f := range m.Fields {
		if f.Desc.Kind() != protoreflect.MessageKind || f.Desc.IsList() || f.Desc.IsMap() {
			continue
		}
		if f.Message == nil {
			continue
		}
		if protoreach.IsWellKnown(f.Message.Desc) {
			continue
		}
		if !protoreach.NeedsCascade(f.Message.Desc, pred, map[protoreflect.FullName]bool{}) {
			continue
		}
		if f.Desc.ContainingOneof() != nil {
			oneof = append(oneof, f)
		} else {
			regular = append(regular, f)
		}
	}
	return
}

func emitApplyDefaults(g *protogen.GeneratedFile, m *protogen.Message, entries []defaultEntry, regular, oneof []*protogen.Field) error {
	goType := m.GoIdent.GoName
	g.P("// ApplyDefaults populates zero-valued fields on ", goType, " with the literal")
	g.P("// defaults declared via (servora.conf.v1.field) annotations, then cascades")
	g.P("// into nested messages that themselves declare defaults.")
	g.P("func (m *", goType, ") ApplyDefaults() {")
	g.P("	if m == nil {")
	g.P("		return")
	g.P("	}")
	for _, e := range entries {
		if err := emitDefaultAssignment(g, e.field, e.rule.GetDefault()); err != nil {
			return fmt.Errorf("field %s: %w", e.field.Desc.FullName(), err)
		}
	}
	for _, f := range regular {
		childType := f.Message.GoIdent.GoName
		g.P("	if m.", f.GoName, " == nil {")
		g.P("		m.", f.GoName, " = &", childType, "{}")
		g.P("	}")
		g.P("	m.", f.GoName, ".ApplyDefaults()")
	}
	for _, f := range oneof {
		g.P("	if v := m.Get", f.GoName, "(); v != nil {")
		g.P("		v.ApplyDefaults()")
		g.P("	}")
	}
	g.P("}")
	g.P()
	return nil
}

// emitDefaultAssignment writes an "if zero, assign literal" block for one
// field. The exact predicate and the literal expression depend on the proto
// field kind.
func emitDefaultAssignment(g *protogen.GeneratedFile, f *protogen.Field, literal string) error {
	goName := f.GoName

	// repeated standard scalars: comma-split literal.
	if f.Desc.IsList() {
		if !isScalarKind(f.Desc.Kind()) {
			return fmt.Errorf("repeated default only supported for scalar element kinds, got %s", f.Desc.Kind())
		}
		expr, err := repeatedScalarLiteral(f.Desc.Kind(), literal)
		if err != nil {
			return err
		}
		g.P("	if len(m.", goName, ") == 0 {")
		g.P("		m.", goName, " = ", expr)
		g.P("	}")
		return nil
	}

	// google.protobuf.Duration → *durationpb.Duration nil-check + set.
	if f.Desc.Kind() == protoreflect.MessageKind &&
		f.Desc.Message() != nil &&
		f.Desc.Message().FullName() == "google.protobuf.Duration" {
		d, err := time.ParseDuration(literal)
		if err != nil {
			return fmt.Errorf("invalid duration default %q: %w", literal, err)
		}
		durationpbNew := g.QualifiedGoIdent(protogen.GoIdent{
			GoName:       "New",
			GoImportPath: "google.golang.org/protobuf/types/known/durationpb",
		})
		g.P("	if m.", goName, " == nil {")
		g.P("		m.", goName, " = ", durationpbNew, "(", strconv.FormatInt(int64(d), 10), ") // ", literal)
		g.P("	}")
		return nil
	}

	// Scalar kinds.
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		g.P("	if m.", goName, ` == "" {`)
		g.P("		m.", goName, " = ", strconv.Quote(literal))
		g.P("	}")
	case protoreflect.BoolKind:
		v, err := strconv.ParseBool(literal)
		if err != nil {
			return fmt.Errorf("invalid bool default %q: %w", literal, err)
		}
		g.P("	if !m.", goName, " {")
		g.P("		m.", goName, " = ", strconv.FormatBool(v))
		g.P("	}")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		v, err := strconv.ParseInt(literal, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid int32 default %q: %w", literal, err)
		}
		g.P("	if m.", goName, " == 0 {")
		g.P("		m.", goName, " = ", strconv.FormatInt(v, 10))
		g.P("	}")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		v, err := strconv.ParseInt(literal, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int64 default %q: %w", literal, err)
		}
		g.P("	if m.", goName, " == 0 {")
		g.P("		m.", goName, " = ", strconv.FormatInt(v, 10))
		g.P("	}")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		v, err := strconv.ParseUint(literal, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid uint32 default %q: %w", literal, err)
		}
		g.P("	if m.", goName, " == 0 {")
		g.P("		m.", goName, " = ", strconv.FormatUint(v, 10))
		g.P("	}")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		v, err := strconv.ParseUint(literal, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint64 default %q: %w", literal, err)
		}
		g.P("	if m.", goName, " == 0 {")
		g.P("		m.", goName, " = ", strconv.FormatUint(v, 10))
		g.P("	}")
	case protoreflect.FloatKind:
		v, err := strconv.ParseFloat(literal, 32)
		if err != nil {
			return fmt.Errorf("invalid float default %q: %w", literal, err)
		}
		g.P("	if m.", goName, " == 0 {")
		g.P("		m.", goName, " = ", strconv.FormatFloat(v, 'g', -1, 32))
		g.P("	}")
	case protoreflect.DoubleKind:
		v, err := strconv.ParseFloat(literal, 64)
		if err != nil {
			return fmt.Errorf("invalid double default %q: %w", literal, err)
		}
		g.P("	if m.", goName, " == 0 {")
		g.P("		m.", goName, " = ", strconv.FormatFloat(v, 'g', -1, 64))
		g.P("	}")
	default:
		return fmt.Errorf("unsupported field kind %s for default literal", f.Desc.Kind())
	}
	return nil
}

func repeatedScalarLiteral(kind protoreflect.Kind, literal string) (string, error) {
	parts := strings.Split(literal, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	var elemType string
	var formatted []string
	switch kind {
	case protoreflect.StringKind:
		elemType = "string"
		for _, p := range parts {
			formatted = append(formatted, strconv.Quote(p))
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		elemType = "int32"
		for _, p := range parts {
			v, err := strconv.ParseInt(p, 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid int32 element %q: %w", p, err)
			}
			formatted = append(formatted, strconv.FormatInt(v, 10))
		}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		elemType = "int64"
		for _, p := range parts {
			v, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid int64 element %q: %w", p, err)
			}
			formatted = append(formatted, strconv.FormatInt(v, 10))
		}
	default:
		return "", fmt.Errorf("repeated default for kind %s not supported", kind)
	}
	return fmt.Sprintf("[]%s{%s}", elemType, strings.Join(formatted, ", ")), nil
}

func isScalarKind(k protoreflect.Kind) bool {
	switch k {
	case protoreflect.StringKind,
		protoreflect.BoolKind,
		protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		return true
	}
	return false
}

// emitCheckRequired generates the CheckRequired() error method. For messages
// with direct required fields it checks zero-values; for cascade containers
// it recurses into children. Oneof children use getter access (m.GetX()).
//
// Nil semantics: messages with direct required fields return error on nil
// receiver (caller should have allocated); cascade-only containers return nil.
func emitCheckRequired(g *protogen.GeneratedFile, m *protogen.Message, fields []*protogen.Field, regular, oneof []*protogen.Field) {
	goType := m.GoIdent.GoName
	hasDirectRequired := len(fields) > 0
	fmtErrorf := g.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Errorf",
		GoImportPath: "fmt",
	})

	g.P("// CheckRequired reports the first required-but-missing field on ", goType, ".")
	g.P("// Fields marked (servora.conf.v1.field) = { required: true } must have a")
	g.P("// non-zero value once configuration loading completes.")
	g.P("func (m *", goType, ") CheckRequired() error {")
	if hasDirectRequired {
		g.P("	if m == nil {")
		g.P(`		return `, fmtErrorf, `("`, m.Desc.FullName(), `: nil receiver")`)
		g.P("	}")
	} else {
		g.P("	if m == nil {")
		g.P("		return nil")
		g.P("	}")
	}
	for _, f := range fields {
		errPath := strings.ToLower(string(m.Desc.FullName())) + "." + jsonNameOrSnake(f)
		predicate, ok := zeroPredicate(f)
		if !ok {
			g.P("	// (skipped: kind ", f.Desc.Kind(), " not supported for required)")
			continue
		}
		g.P("	if ", predicate, " {")
		g.P(`		return `, fmtErrorf, `("`, errPath, ` is required")`)
		g.P("	}")
	}
	for _, f := range regular {
		g.P("	if m.", f.GoName, " != nil {")
		g.P("		if err := m.", f.GoName, ".CheckRequired(); err != nil {")
		g.P("			return err")
		g.P("		}")
		g.P("	}")
	}
	for _, f := range oneof {
		g.P("	if v := m.Get", f.GoName, "(); v != nil {")
		g.P("		if err := v.CheckRequired(); err != nil {")
		g.P("			return err")
		g.P("		}")
		g.P("	}")
	}
	g.P("	return nil")
	g.P("}")
	g.P()
}

// zeroPredicate returns the Go expression that yields true when the field
// holds its zero value, plus whether the field kind supports required-check.
func zeroPredicate(f *protogen.Field) (string, bool) {
	goName := f.GoName
	if f.Desc.IsList() || f.Desc.IsMap() {
		return fmt.Sprintf("len(m.%s) == 0", goName), true
	}
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return fmt.Sprintf(`m.%s == ""`, goName), true
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return fmt.Sprintf("m.%s == nil", goName), true
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		return fmt.Sprintf("m.%s == 0", goName), true
	case protoreflect.BytesKind:
		return fmt.Sprintf("len(m.%s) == 0", goName), true
	}
	return "", false
}

func jsonNameOrSnake(f *protogen.Field) string {
	if j := string(f.Desc.JSONName()); j != "" {
		return camelToSnake(j)
	}
	return string(f.Desc.Name())
}

func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// emitApplyConf generates the composite ApplyConf() error method that
// orchestrates all conf-plugin contracts in the canonical order. The runtime
// calls this single method via the ConfApplier interface; adding future
// capabilities only requires updating this emitter, not the runtime.
func emitApplyConf(g *protogen.GeneratedFile, m *protogen.Message, hasRequired, hasDefaults bool) {
	goType := m.GoIdent.GoName
	g.P("// ApplyConf runs the full post-scan conf contract sequence on ", goType, ".")
	g.P("// Generated by protoc-gen-servora-conf; the runtime calls this via ConfApplier.")
	g.P("func (m *", goType, ") ApplyConf() error {")
	if hasRequired {
		g.P("	if err := m.CheckRequired(); err != nil {")
		g.P("		return err")
		g.P("	}")
	}
	if hasDefaults {
		g.P("	m.ApplyDefaults()")
	}
	g.P("	return nil")
	g.P("}")
	g.P()
}

// Ensure the file path helper compiles even if unused in some build flavors.
var _ = path.Dir

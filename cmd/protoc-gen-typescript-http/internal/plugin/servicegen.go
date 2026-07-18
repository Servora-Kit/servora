package plugin

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/codegen"
	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type serviceGenerator struct {
	pkg     protoreflect.FullName
	service protoreflect.ServiceDescriptor
}

func (s serviceGenerator) Generate(f *codegen.File) error {
	s.generateInterface(f)
	return s.generateClient(f)
}

func (s serviceGenerator) generateInterface(f *codegen.File) {
	commentGenerator{descriptor: s.service}.generateLeading(f, 0)
	f.P("export interface ", descriptorTypeName(s.service), " {")
	rangeMethods(s.service.Methods(), func(method protoreflect.MethodDescriptor) {
		ok, reason := supportedMethod(method)
		if !ok {
			Warn("method %s.%s skipped: %s", s.service.FullName(), method.Name(), reason)
			return
		}
		if isStreamingMethod(method) {
			r, ok := httprule.Get(method)
			if !ok {
				Warn("streaming method %s.%s has no http rule; skipping", s.service.FullName(), method.Name())
				return
			}
			rule, err := httprule.ParseRule(r)
			if err != nil {
				Warn("streaming method %s.%s has invalid http rule: %v; skipping", s.service.FullName(), method.Name(), err)
				return
			}
			generateStreamInterfaceMethod(f, s.pkg, method, rule)
			return
		}
		commentGenerator{descriptor: method}.generateLeading(f, 1)
		input := typeFromMessage(s.pkg, method.Input())
		output := typeFromMessage(s.pkg, method.Output())
		f.P(t(1), method.Name(), "(")
		f.P(t(2), "request: ", input.Reference(), ",")
		f.P(t(1), "): Promise<", output.Reference(), ">;")
	})
	f.P("}")
	f.P()
}

func (s serviceGenerator) generateClient(f *codegen.File) error {
	f.P(
		"export function create",
		descriptorTypeName(s.service),
		"Client(",
		"\n",
		t(1),
		"transport: ClientTransport,",
		"\n",
		"): ",
		descriptorTypeName(s.service),
		" {",
	)
	f.P(t(1), "return {")
	var methodErrs []error
	rangeMethods(s.service.Methods(), func(method protoreflect.MethodDescriptor) {
		ok, reason := supportedMethod(method)
		if !ok {
			Warn("method %s.%s skipped in client: %s", s.service.FullName(), method.Name(), reason)
			return
		}
		if err := s.generateMethod(f, method); err != nil {
			methodErrs = append(methodErrs, fmt.Errorf("generate method %s.%s: %w", s.service.FullName(), method.Name(), err))
		}
	})
	if len(methodErrs) > 0 {
		return fmt.Errorf("%d method(s) failed: %w", len(methodErrs), errors.Join(methodErrs...))
	}
	f.P(t(1), "};")
	f.P("}")
	return nil
}

func (s serviceGenerator) generateMethod(f *codegen.File, method protoreflect.MethodDescriptor) error {
	outputType := typeFromMessage(s.pkg, method.Output())
	r, ok := httprule.Get(method)
	if !ok {
		Warn("method %s.%s has no http rule annotation; skipping", s.service.FullName(), method.Name())
		return nil
	}
	rule, err := httprule.ParseRule(r)
	if err != nil {
		return fmt.Errorf("parse http rule: %w", err)
	}
	if isStreamingMethod(method) {
		generateStreamClientMethod(f, s.pkg, method, rule)
		return nil
	}
	paramName := "request"
	if !methodUsesRequest(rule, method.Input()) {
		paramName = "_request"
	}
	f.P(t(2), method.Name(), "(", paramName, ") {")
	generateMethodPathValidation(f, method.Input(), rule)
	generateMethodPath(f, method.Input(), rule)
	generateMethodBody(f, method.Input(), rule)
	hasQP := generateMethodQuery(f, method.Input(), rule)
	uriVar := "path"
	if hasQP {
		f.P(t(3), "let uri = path;")
		f.P(t(3), "if (queryParams.length > 0) {")
		f.P(t(4), "uri += `?${queryParams.join('&')}`;")
		f.P(t(3), "}")
		uriVar = "uri"
	}
	f.P(t(3), "return transport.unary(", uriVar, ", ", tsSingleQuote(rule.Method), ", body, {")
	f.P(t(4), "service: '", method.Parent().Name(), "',")
	f.P(t(4), "method: '", method.Name(), "',")
	f.P(t(3), "}) as Promise<", outputType.Reference(), ">;")
	f.P(t(2), "},")
	return nil
}

func generateMethodPathValidation(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) {
	for _, seg := range rule.Template.Segments {
		if seg.Kind != httprule.SegmentKindVariable {
			continue
		}
		fp := seg.Variable.FieldPath
		nullPath := nullPropagationPath(fp, input)
		protoPath := strings.Join(fp, ".")
		errMsg := "missing required field request." + protoPath
		f.P(t(3), "if (request.", nullPath, " === undefined || request.", nullPath, " === null) {")
		f.P(t(4), "throw new Error(", tsSingleQuote(errMsg), ");")
		f.P(t(3), "}")
	}
}

func generateMethodPath(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) {
	pathParts := make([]string, 0, len(rule.Template.Segments))
	for _, seg := range rule.Template.Segments {
		switch seg.Kind {
		case httprule.SegmentKindVariable:
			fieldPath := jsonPath(seg.Variable.FieldPath, input)
			encoder := "encodePathSegment"
			if isMultiSegmentPathVariable(seg.Variable) {
				encoder = "encodeMultiSegmentPath"
			}
			pathParts = append(pathParts, "${"+encoder+"(request."+fieldPath+")}")
		case httprule.SegmentKindLiteral:
			pathParts = append(pathParts, escapeTemplateLiteral(seg.Literal))
		case httprule.SegmentKindMatchSingle:
			pathParts = append(pathParts, "*")
		case httprule.SegmentKindMatchMultiple:
			pathParts = append(pathParts, "**")
		}
	}
	path := strings.Join(pathParts, "/")
	if rule.Template.Verb != "" {
		path += ":" + escapeTemplateLiteral(rule.Template.Verb)
	}
	f.P(t(3), "const path = `", path, "`;")
}

func isMultiSegmentPathVariable(variable httprule.VariableSegment) bool {
	return len(variable.Segments) != 1 || variable.Segments[0].Kind == httprule.SegmentKindMatchMultiple
}

// escapeTemplateLiteral escapes characters that have special meaning inside a
// JavaScript template literal (backtick string) to prevent generated code
// injection and syntax errors.
func escapeTemplateLiteral(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "${", "\\${")
	return s
}

func generateMethodBody(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) {
	switch {
	case rule.Body == "":
		f.P(t(3), "const body = null;")
	case rule.Body == "*":
		f.P(t(3), "const body = JSON.stringify(request);")
	default:
		bodyField := input.Fields().ByName(protoreflect.Name(rule.Body))
		if bodyField == nil {
			Warn("body field %q referenced in http rule not found in message %s; falling back to full request", rule.Body, input.FullName())
			f.P(t(3), "const body = JSON.stringify(request);")
			return
		}
		nullPath := nullPropagationPath(httprule.FieldPath{rule.Body}, input)
		f.P(t(3), "const body = JSON.stringify(request?.", nullPath, " ?? {});")
	}
}

// methodUsesRequest returns true if the generated method body will reference
// the request parameter (in path, body, or query params).
func methodUsesRequest(rule httprule.Rule, input protoreflect.MessageDescriptor) bool {
	return hasPathVariables(rule) || rule.Body != "" || hasQueryParams(input, rule)
}

// hasQueryParams returns true if the method has fields that will be rendered
// as query parameters.
func hasQueryParams(input protoreflect.MessageDescriptor, rule httprule.Rule) bool {
	if rule.Body == "*" {
		return false
	}
	pathCovered := make(map[string]struct{})
	for _, segment := range rule.Template.Segments {
		if segment.Kind != httprule.SegmentKindVariable {
			continue
		}
		pathCovered[segment.Variable.FieldPath.String()] = struct{}{}
	}
	found := false
	walkJSONLeafFields(input, func(path httprule.FieldPath, field protoreflect.FieldDescriptor) {
		if found {
			return
		}
		if len(path) == 0 || isPathCovered(path, pathCovered) || isBodyField(path, rule) {
			return
		}
		found = true
	})
	return found
}

func generateMethodQuery(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) bool {
	if !hasQueryParams(input, rule) {
		return false
	}
	pathCovered := make(map[string]struct{})
	for _, segment := range rule.Template.Segments {
		if segment.Kind != httprule.SegmentKindVariable {
			continue
		}
		pathCovered[segment.Variable.FieldPath.String()] = struct{}{}
	}
	f.P(t(3), "const queryParams: string[] = [];")
	walkJSONLeafFields(input, func(path httprule.FieldPath, field protoreflect.FieldDescriptor) {
		if len(path) == 0 || isPathCovered(path, pathCovered) || isBodyField(path, rule) {
			return
		}
		nullPath := nullPropagationPath(path, input)
		jp := jsonPath(path, input)
		f.P(t(3), "if (request.", nullPath, ") {")
		switch {
		case field.IsMap():
			f.P(t(4), "Object.entries(request.", jp, ").forEach(([key, value]) => {")
			f.P(t(5), "queryParams.push(")
			f.P(t(6), "`", jp, "[key]=${encodeURIComponent(value.toString())}`,")
			f.P(t(5), ");")
			f.P(t(4), "});")
		case field.IsList():
			f.P(t(4), "request.", jp, ".forEach((x) => {")
			f.P(t(5), "queryParams.push(")
			f.P(t(6), "`", jp, "=${encodeURIComponent(x.toString())}`,")
			f.P(t(5), ");")
			f.P(t(4), "});")
		default:
			f.P(t(4), "queryParams.push(")
			f.P(t(5), "`", jp, "=${encodeURIComponent(request.", jp, ".toString())}`,")
			f.P(t(4), ");")
		}
		f.P(t(3), "}")
	})
	return true
}

func isPathCovered(path httprule.FieldPath, covered map[string]struct{}) bool {
	_, ok := covered[path.String()]
	return ok
}

func isBodyField(path httprule.FieldPath, rule httprule.Rule) bool {
	return rule.Body != "" && path[0] == rule.Body
}

// supportedMethod returns whether a method is supported by this generator,
// along with a human-readable reason if it is not.
func supportedMethod(method protoreflect.MethodDescriptor) (bool, string) {
	_, ok := httprule.Get(method)
	if !ok {
		return false, "no http rule annotation (google.api.http)"
	}
	if method.IsStreamingClient() && !method.IsStreamingServer() {
		return false, "client-only streaming is not supported"
	}
	return true, ""
}

func jsonPath(path httprule.FieldPath, message protoreflect.MessageDescriptor) string {
	return strings.Join(jsonPathSegments(path, message), ".")
}

func nullPropagationPath(path httprule.FieldPath, message protoreflect.MessageDescriptor) string {
	return strings.Join(jsonPathSegments(path, message), "?.")
}

func jsonPathSegments(path httprule.FieldPath, message protoreflect.MessageDescriptor) []string {
	segs := make([]string, len(path))
	for i, p := range path {
		field := message.Fields().ByName(protoreflect.Name(p))
		if field == nil {
			Warn("field %q not found in message %s; path segment may be incorrect", p, message.FullName())
			segs[i] = p
			continue
		}
		segs[i] = field.JSONName()
		if i < len(path)-1 {
			if field.Kind() != protoreflect.MessageKind {
				Warn("field %q in message %s is not a message type; cannot traverse nested path %s", p, message.FullName(), path.String())
				break
			}
			nested := field.Message()
			if nested == nil {
				Warn("field %q in message %s has no valid message descriptor; cannot traverse nested path", p, message.FullName())
				break
			}
			message = nested
		}
	}
	return segs
}

package plugin

import (
	"strings"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/codegen"
	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type transportUsage struct {
	clientTransport        bool
	serverStream           bool
	duplexStream           bool
	encodePathSegment      bool
	encodeMultiSegmentPath bool
}

func serviceHasSupportedHTTPMethod(svc protoreflect.ServiceDescriptor) bool {
	methods := svc.Methods()
	for i := range methods.Len() {
		if ok, _ := supportedMethod(methods.Get(i)); ok {
			return true
		}
	}
	return false
}

func collectTransportUsage(files []protoreflect.FileDescriptor) transportUsage {
	var usage transportUsage
	for _, file := range files {
		services := file.Services()
		for i := range services.Len() {
			svc := services.Get(i)
			if !serviceHasSupportedHTTPMethod(svc) {
				continue
			}
			usage.clientTransport = true
			rangeMethods(svc.Methods(), func(method protoreflect.MethodDescriptor) {
				if ok, _ := supportedMethod(method); !ok {
					return
				}
				r, ok := httprule.Get(method)
				if !ok {
					return
				}
				rule, err := httprule.ParseRule(r)
				if err != nil {
					return
				}
				if method.IsStreamingClient() {
					usage.duplexStream = true
				} else if method.IsStreamingServer() {
					usage.serverStream = true
				}
				for _, seg := range rule.Template.Segments {
					if seg.Kind != httprule.SegmentKindVariable {
						continue
					}
					if isMultiSegmentPathVariable(seg.Variable) {
						usage.encodeMultiSegmentPath = true
					} else {
						usage.encodePathSegment = true
					}
				}
			})
		}
	}
	return usage
}

func generateTransportInfra(f *codegen.File, files []protoreflect.FileDescriptor, defaultHost string) {
	usage := collectTransportUsage(files)
	var typeImports []string
	if usage.clientTransport {
		typeImports = append(typeImports, "ClientTransport")
	}
	if usage.duplexStream {
		typeImports = append(typeImports, "DuplexStream")
	}
	if usage.serverStream {
		typeImports = append(typeImports, "ServerStream")
	}
	var valueImports []string
	if usage.encodeMultiSegmentPath {
		valueImports = append(valueImports, "encodeMultiSegmentPath")
	}
	if usage.encodePathSegment {
		valueImports = append(valueImports, "encodePathSegment")
	}
	if len(typeImports) > 0 {
		f.P("import type { ", strings.Join(typeImports, ", "), " } from '@servora/proto-utils/transport';")
	}
	if len(valueImports) > 0 {
		f.P("import { ", strings.Join(valueImports, ", "), " } from '@servora/proto-utils/transport';")
	}
	if len(typeImports) > 0 || len(valueImports) > 0 {
		f.P()
	}
	if defaultHost != "" {
		f.P("export const DEFAULT_HOST = ", tsSingleQuote(defaultHost), ";")
		f.P()
	}
}

func generateStreamInterfaceMethod(f *codegen.File, pkg protoreflect.FullName, method protoreflect.MethodDescriptor, rule httprule.Rule) {
	commentGenerator{descriptor: method}.generateLeading(f, 1)
	input := typeFromMessage(pkg, method.Input())
	output := typeFromMessage(pkg, method.Output())
	if method.IsStreamingClient() {
		if bidiStreamUsesRequest(rule, method.Input()) {
			f.P(t(1), method.Name(), "(")
			f.P(t(2), "request: ", input.Reference(), ",")
			f.P(t(1), "): DuplexStream<", input.Reference(), ", ", output.Reference(), ">;")
		} else {
			f.P(t(1), method.Name(), "(): DuplexStream<", input.Reference(), ", ", output.Reference(), ">;")
		}
	} else {
		f.P(t(1), method.Name(), "(")
		f.P(t(2), "request: ", input.Reference(), ",")
		f.P(t(1), "): ServerStream<", output.Reference(), ">;")
	}
}

func generateStreamClientMethod(
	f *codegen.File,
	pkg protoreflect.FullName,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) {
	if method.IsStreamingClient() {
		generateBidiStreamMethod(f, pkg, method, rule)
	} else {
		generateServerStreamMethod(f, pkg, method, rule)
	}
}

func generateServerStreamMethod(
	f *codegen.File,
	pkg protoreflect.FullName,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) {
	output := typeFromMessage(pkg, method.Output())
	paramName := "request"
	if !methodUsesRequest(rule, method.Input()) {
		paramName = "_request"
	}
	f.P(t(2), method.Name(), "(", paramName, ") {")
	generateMethodPathValidation(f, method.Input(), rule)
	generateMethodPath(f, method.Input(), rule)
	hasQP := generateMethodQuery(f, method.Input(), rule)
	uriVar := "path"
	if hasQP {
		f.P(t(3), "let uri = path;")
		f.P(t(3), "if (queryParams.length > 0) {")
		f.P(t(4), "uri += `?${queryParams.join('&')}`;")
		f.P(t(3), "}")
		uriVar = "uri"
	}
	f.P(t(3), "return transport.serverStream<", output.Reference(), ">(", uriVar, ", {")
	f.P(t(4), "service: '", method.Parent().Name(), "',")
	f.P(t(4), "method: '", method.Name(), "',")
	f.P(t(3), "});")
	f.P(t(2), "},")
}

func generateBidiStreamMethod(
	f *codegen.File,
	pkg protoreflect.FullName,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) {
	input := typeFromMessage(pkg, method.Input())
	output := typeFromMessage(pkg, method.Output())
	usesRequest := bidiStreamUsesRequest(rule, method.Input())
	if usesRequest {
		f.P(t(2), method.Name(), "(request) {")
		generateMethodPathValidation(f, method.Input(), rule)
	} else {
		f.P(t(2), method.Name(), "() {")
	}
	generateMethodPath(f, method.Input(), rule)
	uriVar := generateStreamURI(f, method.Input(), rule)
	generateBidiStreamReturn(f, method, rule, input, output, uriVar)
	f.P(t(2), "},")
}

func generateStreamURI(
	f *codegen.File,
	input protoreflect.MessageDescriptor,
	rule httprule.Rule,
) string {
	if !generateMethodQuery(f, input, rule) {
		return "path"
	}
	f.P(t(3), "let uri = path;")
	f.P(t(3), "if (queryParams.length > 0) {")
	f.P(t(4), "uri += `?${queryParams.join('&')}`;")
	f.P(t(3), "}")
	return "uri"
}

func generateBidiStreamReturn(
	f *codegen.File,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
	input Type,
	output Type,
	uriVar string,
) {
	f.P(t(3), "return transport.duplexStream<", input.Reference(), ", ", output.Reference(), ">(", uriVar, ", {")
	f.P(t(4), "service: '", method.Parent().Name(), "',")
	f.P(t(4), "method: '", method.Name(), "',")
	if bodyPath := streamBodyJSONPath(method.Input(), rule); bodyPath != "" {
		f.P(t(3), "}, (data) => data.", bodyPath, ");")
		return
	}
	f.P(t(3), "});")
}

func streamBodyJSONPath(input protoreflect.MessageDescriptor, rule httprule.Rule) string {
	if rule.Body == "" || rule.Body == "*" {
		return ""
	}
	bodyField := input.Fields().ByName(protoreflect.Name(rule.Body))
	if bodyField == nil {
		Warn("流式 HTTP 规则引用的 body 字段 %q 不存在于消息 %s，回退为完整请求", rule.Body, input.FullName())
		return ""
	}
	if bodyField.Kind() != protoreflect.MessageKind || bodyField.IsList() || bodyField.IsMap() {
		return ""
	}
	return bodyField.JSONName()
}

func bidiStreamUsesRequest(rule httprule.Rule, input protoreflect.MessageDescriptor) bool {
	return hasPathVariables(rule) || hasQueryParams(input, rule)
}

func hasPathVariables(rule httprule.Rule) bool {
	for _, seg := range rule.Template.Segments {
		if seg.Kind == httprule.SegmentKindVariable {
			return true
		}
	}
	return false
}

func isStreamingMethod(method protoreflect.MethodDescriptor) bool {
	return method.IsStreamingClient() || method.IsStreamingServer()
}

// getDefaultHost reads the google.api.default_host extension from a service descriptor.
func getDefaultHost(service protoreflect.ServiceDescriptor) string {
	if service.Options() == nil {
		return ""
	}
	ext := proto.GetExtension(service.Options(), annotations.E_DefaultHost)
	if host, ok := ext.(string); ok && host != "" {
		return host
	}
	return ""
}

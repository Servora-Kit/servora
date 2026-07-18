package plugin

import (
	"strings"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/codegen"
	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/httprule"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func generateTransportInfra(f *codegen.File, defaultHost string) {
	f.P("interface TransportMeta {")
	f.P(t(1), "service: string;")
	f.P(t(1), "method: string;")
	f.P("}")
	f.P()
	f.P("export interface ClientTransport {")
	f.P(t(1), "unary(")
	f.P(t(2), "path: string,")
	f.P(t(2), "method: string,")
	f.P(t(2), "body: null | string,")
	f.P(t(2), "meta: TransportMeta,")
	f.P(t(1), "): Promise<unknown>;")
	f.P(t(1), "serverStream<T>(path: string, meta: TransportMeta): ServerStream<T>;")
	f.P(t(1), "duplexStream<TIn, TOut>(path: string, meta: TransportMeta): DuplexStream<TIn, TOut>;")
	f.P("}")
	f.P()
	f.P("export interface ServerStream<T> {")
	f.P(t(1), "onEvent(listener: (data: T) => void): () => void;")
	f.P(t(1), "onError(handler: (error: Error) => void): void;")
	f.P(t(1), "close(): void;")
	f.P("}")
	f.P()
	f.P("export interface DuplexStream<TIn, TOut> extends ServerStream<TOut> {")
	f.P(t(1), "send(data: TIn): void;")
	f.P("}")
	f.P()
	f.P("function encodePathSegment(value: unknown): string {")
	f.P(t(1), "return encodeURIComponent(String(value)).replace(/[!'()*]/g, (character) => `%${character.charCodeAt(0).toString(16).toUpperCase()}`);")
	f.P("}")
	f.P()
	f.P("function encodeMultiSegmentPath(value: unknown): string {")
	f.P(t(1), "return String(value)")
	f.P(t(2), ".split('/')")
	f.P(t(2), ".map(encodePathSegment)")
	f.P(t(2), ".join('/');")
	f.P("}")
	f.P()
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
		if hasPathVariables(rule) {
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
		generateBidiStreamMethod(f, method, rule)
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
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) {
	if hasPathVariables(rule) {
		generateBidiStreamWithParams(f, method, rule)
	} else {
		generateBidiStreamLiteral(f, method, rule)
	}
}

func generateBidiStreamLiteral(
	f *codegen.File,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) {
	path := literalPath(rule)
	f.P(t(2), method.Name(), "() {")
	f.P(t(3), "const path = ", path, ";")
	f.P(t(3), "return transport.duplexStream(path, {")
	f.P(t(4), "service: '", method.Parent().Name(), "',")
	f.P(t(4), "method: '", method.Name(), "',")
	f.P(t(3), "});")
	f.P(t(2), "},")
}

func generateBidiStreamWithParams(
	f *codegen.File,
	method protoreflect.MethodDescriptor,
	rule httprule.Rule,
) {
	f.P(t(2), method.Name(), "(request) {")
	generateMethodPathValidation(f, method.Input(), rule)
	generateMethodPath(f, method.Input(), rule)
	f.P(t(3), "return transport.duplexStream(path, {")
	f.P(t(4), "service: '", method.Parent().Name(), "',")
	f.P(t(4), "method: '", method.Name(), "',")
	f.P(t(3), "});")
	f.P(t(2), "},")
}

func literalPath(rule httprule.Rule) string {
	parts := make([]string, 0, len(rule.Template.Segments))
	for _, seg := range rule.Template.Segments {
		switch seg.Kind {
		case httprule.SegmentKindLiteral:
			parts = append(parts, seg.Literal)
		case httprule.SegmentKindMatchSingle:
			parts = append(parts, "*")
		case httprule.SegmentKindMatchMultiple:
			parts = append(parts, "**")
		}
	}
	path := strings.Join(parts, "/")
	if rule.Template.Verb != "" {
		path += ":" + rule.Template.Verb
	}
	return tsSingleQuote(path)
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

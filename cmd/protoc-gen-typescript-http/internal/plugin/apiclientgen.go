package plugin

import (
	"sort"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/codegen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// generateApiClient generates a unified ApiClient class that aggregates all
// service clients into a single entry point. Users only need to inject one
// transport to access every service.
//
// The generated class uses lazy initialization so that each service client
// is only created on first access.
//
// Example generated output:
//
//	export class ApiClient {
//	  private readonly _transport: ClientTransport;
//	  private _freightService?: FreightService;
//	  constructor(transport: ClientTransport) {
//	    this._transport = transport;
//	  }
//	  get freightService(): FreightService {
//	    return this._freightService ??= createFreightServiceClient(this._transport);
//	  }
//	}
//
//	export function createApiClient(transport: ClientTransport): ApiClient {
//	  return new ApiClient(transport);
//	}
func generateApiClient(f *codegen.File, services []protoreflect.ServiceDescriptor) {
	if len(services) == 0 {
		Warn("no services found in package; skipping ApiClient generation")
		return
	}

	// Class declaration
	f.P("export class ApiClient {")

	// Collect all private fields and sort alphabetically by name
	type classField struct {
		declaration string
		name        string
	}
	fields := []classField{
		{declaration: t(1) + "private readonly _transport: ClientTransport;", name: "_transport"},
	}
	for _, svc := range services {
		typeName := descriptorTypeName(svc)
		fieldName := lowerFirst(string(svc.Name()))
		fields = append(fields, classField{
			declaration: t(1) + "private _" + fieldName + "?: " + typeName + ";",
			name:        "_" + fieldName,
		})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].name < fields[j].name
	})
	for _, fld := range fields {
		f.P(fld.declaration)
	}
	f.P()

	// Constructor
	f.P(t(1), "constructor(transport: ClientTransport) {")
	f.P(t(2), "this._transport = transport;")
	f.P(t(1), "}")
	f.P()

	// Getter for each service (sorted alphabetically by field name)
	sortedServices := make([]protoreflect.ServiceDescriptor, len(services))
	copy(sortedServices, services)
	sort.Slice(sortedServices, func(i, j int) bool {
		return lowerFirst(string(sortedServices[i].Name())) < lowerFirst(string(sortedServices[j].Name()))
	})
	for i, svc := range sortedServices {
		typeName := descriptorTypeName(svc)
		serviceName := string(svc.Name())
		fieldName := lowerFirst(serviceName)
		f.P(t(1), "get ", fieldName, "(): ", typeName, " {")
		f.P(t(2), "return this._", fieldName, " ??= create", serviceName, "Client(this._transport);")
		f.P(t(1), "}")
		if i < len(sortedServices)-1 {
			f.P()
		}
	}
	f.P("}")
	f.P()

	// Convenience factory
	f.P("export function createApiClient(transport: ClientTransport): ApiClient {")
	f.P(t(1), "return new ApiClient(transport);")
	f.P("}")
	f.P()
}

// lowerFirst returns s with its first character lowercased.
func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	bytes := []byte(s)
	if bytes[0] >= 'A' && bytes[0] <= 'Z' {
		bytes[0] += 32
	}
	return string(bytes)
}

package crud

import (
	"math"
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type dynamicFilterResource struct {
	descriptor protoreflect.MessageDescriptor
	fields     map[string]FieldPlan
	paths      []string
}

func (resource dynamicFilterResource) ResourceType() string { return "test.dev/Resource" }
func (resource dynamicFilterResource) Descriptor() protoreflect.MessageDescriptor {
	return resource.descriptor
}
func (resource dynamicFilterResource) Field(path string) (FieldPlan, bool) {
	field, ok := resource.fields[path]
	return field, ok
}
func (resource dynamicFilterResource) QueryablePaths() []string {
	return append([]string(nil), resource.paths...)
}

func TestParseFilterSupportsBoolAndRepeatedContainment(t *testing.T) {
	t.Parallel()

	resource := newDynamicFilterResource(t)
	tests := []string{
		`enabled = true`,
		`flags:true`,
		`tags:"blue"`,
		`roles:ACTIVE`,
		`display_name = null AND enabled = true`,
	}
	for _, filter := range tests {
		t.Run(filter, func(t *testing.T) {
			t.Parallel()
			depth, err := filterParenthesisDepth(filter)
			if err != nil {
				t.Fatalf("filterParenthesisDepth: %v", err)
			}
			if _, err := parseFilter(filter, resource, depth); err != nil {
				t.Fatalf("parseFilter(%q): %v", filter, err)
			}
		})
	}
}

func TestParseFilterSupportsIntegerBoundaries(t *testing.T) {
	t.Parallel()

	resource := newDynamicFilterResource(t)
	tests := []struct {
		filter string
		kind   FilterValueKind
	}{
		{`count = -9223372036854775808`, FilterValueInt64},
		{`unsigned = 18446744073709551615`, FilterValueUint64},
		{`unsigned = 0xffffffffffffffff`, FilterValueUint64},
	}
	for _, test := range tests {
		t.Run(test.filter, func(t *testing.T) {
			t.Parallel()
			parsed, err := parseFilter(test.filter, resource, 0)
			if err != nil {
				t.Fatalf("parseFilter: %v", err)
			}
			if got := parsed.Root().Value().Kind(); got != test.kind {
				t.Fatalf("value kind = %v, want %v", got, test.kind)
			}
		})
	}

	parsed, err := parseFilter(`unsigned = 18446744073709551615`, resource, 0)
	if err != nil {
		t.Fatalf("parseFilter max uint64: %v", err)
	}
	if got, ok := parsed.Root().Value().Uint64Value(); !ok || got != math.MaxUint64 {
		t.Fatalf("uint64 value = (%d, %v), want (%d, true)", got, ok, uint64(math.MaxUint64))
	}
	if _, err := parseFilter(`unsigned = 18446744073709551616`, resource, 0); err == nil {
		t.Fatal("parseFilter accepted uint64 overflow")
	}
	if _, err := parseFilter(`count = -9223372036854775809`, resource, 0); err == nil {
		t.Fatal("parseFilter accepted int64 underflow")
	}
}

func TestParseFilterRejectsInternalLargeIntegerFunctions(t *testing.T) {
	t.Parallel()

	resource := newDynamicFilterResource(t)
	for _, filter := range []string{
		`count = __servora_int64("7")`,
		`unsigned = __servora_uint64("7")`,
	} {
		if _, err := parseFilter(filter, resource, 0); err == nil {
			t.Fatalf("parseFilter accepted private function in %q", filter)
		}
	}
}

func TestParseFilterNodeCountIgnoresLargeIntegerRewrite(t *testing.T) {
	t.Parallel()

	resource := newDynamicFilterResource(t)
	small, err := parseFilter(`unsigned = 7`, resource, 0)
	if err != nil {
		t.Fatalf("parseFilter small: %v", err)
	}
	large, err := parseFilter(`unsigned = 18446744073709551615`, resource, 0)
	if err != nil {
		t.Fatalf("parseFilter large: %v", err)
	}
	if small.NodeCount() != large.NodeCount() {
		t.Fatalf("node counts differ: small=%d large=%d", small.NodeCount(), large.NodeCount())
	}
}

func TestParseProtoDurationUsesStrictDecimalSyntax(t *testing.T) {
	t.Parallel()

	valid, err := parseProtoDuration("-1.000000001s")
	if err != nil {
		t.Fatalf("parseProtoDuration valid: %v", err)
	}
	if valid.Seconds != -1 || valid.Nanos != -1 {
		t.Fatalf("duration = (%d, %d), want (-1, -1)", valid.Seconds, valid.Nanos)
	}
	for _, value := range []string{"1e3s", "1.0000000001s", "+1s", ".5s", "1.s"} {
		if _, err := parseProtoDuration(value); err == nil {
			t.Fatalf("parseProtoDuration accepted %q", value)
		}
	}
}

func newDynamicFilterResource(t *testing.T) dynamicFilterResource {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    stringPointer("filter_internal_test.proto"),
		Package: stringPointer("crudtest"),
		Syntax:  stringPointer("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: stringPointer("Status"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: stringPointer("STATUS_UNSPECIFIED"), Number: int32Pointer(0)},
				{Name: stringPointer("ACTIVE"), Number: int32Pointer(1)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: stringPointer("Resource"),
			Field: []*descriptorpb.FieldDescriptorProto{
				fieldDescriptor("enabled", 1, descriptorpb.FieldDescriptorProto_TYPE_BOOL, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				fieldDescriptor("flags", 2, descriptorpb.FieldDescriptorProto_TYPE_BOOL, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
				fieldDescriptor("tags", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
				fieldDescriptor("roles", 4, descriptorpb.FieldDescriptorProto_TYPE_ENUM, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ".crudtest.Status"),
				fieldDescriptor("count", 5, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				fieldDescriptor("unsigned", 6, descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				fieldDescriptor("display_name", 7, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
			},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	descriptor := file.Messages().ByName("Resource")
	resource := dynamicFilterResource{
		descriptor: descriptor,
		fields:     make(map[string]FieldPlan),
		paths:      make([]string, 0, descriptor.Fields().Len()),
	}
	for index := 0; index < descriptor.Fields().Len(); index++ {
		field := descriptor.Fields().Get(index)
		path := string(field.Name())
		resource.fields[path] = FieldPlan{path: path, descriptor: field}
		resource.paths = append(resource.paths, path)
	}
	return resource
}

func fieldDescriptor(
	name string,
	number int32,
	fieldType descriptorpb.FieldDescriptorProto_Type,
	label descriptorpb.FieldDescriptorProto_Label,
	typeName string,
) *descriptorpb.FieldDescriptorProto {
	field := &descriptorpb.FieldDescriptorProto{
		Name:   stringPointer(name),
		Number: int32Pointer(number),
		Type:   &fieldType,
		Label:  &label,
	}
	if typeName != "" {
		field.TypeName = stringPointer(typeName)
	}
	return field
}

func stringPointer(value string) *string { return &value }
func int32Pointer(value int32) *int32    { return &value }

package crud

import (
	"slices"
	"strings"
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestLifecycleExpandsParentAndValidatesNestedRequired(t *testing.T) {
	t.Parallel()

	plan, resource := newLifecyclePlan(t)
	setLifecycleString(resource, "profile.bio", "new")
	if _, err := plan.PrepareUpdate(resource, &fieldmaskpb.FieldMask{Paths: []string{"profile"}}, UpdateOptions{}); !crudpb.IsCrudErrorReasonInvalidFieldValue(err) {
		t.Fatalf("PrepareUpdate missing nested REQUIRED error = %v, want INVALID_FIELD_VALUE", err)
	}
	setLifecycleString(resource, "profile.locale", "en")
	prepared, err := plan.PrepareUpdate(resource, &fieldmaskpb.FieldMask{Paths: []string{"profile"}}, UpdateOptions{})
	if err != nil {
		t.Fatalf("PrepareUpdate: %v", err)
	}
	if got, want := prepared.WriteMask().GetPaths(), []string{"profile.bio", "profile.locale"}; !slices.Equal(got, want) {
		t.Fatalf("WriteMask paths = %v, want %v", got, want)
	}
}

func TestLifecycleDistinguishesIndirectImmutablePresence(t *testing.T) {
	t.Parallel()

	plan, resource := newLifecyclePlan(t)
	setLifecycleString(resource, "profile.locale", "en")
	prepared, err := plan.PrepareUpdate(resource, &fieldmaskpb.FieldMask{Paths: []string{"profile"}}, UpdateOptions{})
	if err != nil {
		t.Fatalf("PrepareUpdate absent immutable: %v", err)
	}
	if got := len(prepared.ImmutableComparisons()); got != 0 {
		t.Fatalf("absent indirect immutable comparisons = %d, want 0", got)
	}

	setLifecycleString(resource, "profile.origin", "import")
	prepared, err = plan.PrepareUpdate(resource, &fieldmaskpb.FieldMask{Paths: []string{"profile"}}, UpdateOptions{})
	if err != nil {
		t.Fatalf("PrepareUpdate present immutable: %v", err)
	}
	comparisons := prepared.ImmutableComparisons()
	if got, want := len(comparisons), 1; got != want || comparisons[0].Direct() {
		t.Fatalf("comparisons = %+v, want one indirect comparison", comparisons)
	}
}

func TestCreateNestedRequiredOnlyAppliesWhenOptionalParentPresent(t *testing.T) {
	t.Parallel()

	plan, resource := newLifecyclePlan(t)
	if _, err := plan.PrepareCreate(resource); err != nil {
		t.Fatalf("PrepareCreate absent optional parent: %v", err)
	}
	resource.Mutable(resource.Descriptor().Fields().ByName("profile"))
	if _, err := plan.PrepareCreate(resource); !crudpb.IsCrudErrorReasonInvalidFieldValue(err) {
		t.Fatalf("PrepareCreate present empty parent error = %v, want INVALID_FIELD_VALUE", err)
	}
}

func TestLifecycleKeepsInputOnlyWritesAndIgnoresOutputOnly(t *testing.T) {
	t.Parallel()

	plan, resource := newLifecyclePlan(t)
	setLifecycleString(resource, "secret", "one-shot")
	setLifecycleString(resource, "server_value", "ignored")
	prepared, err := plan.PrepareUpdate(
		resource,
		&fieldmaskpb.FieldMask{Paths: []string{"secret", "server_value"}},
		UpdateOptions{},
	)
	if err != nil {
		t.Fatalf("PrepareUpdate: %v", err)
	}
	if got, want := prepared.WriteMask().GetPaths(), []string{"secret"}; !slices.Equal(got, want) {
		t.Fatalf("WriteMask paths = %v, want %v", got, want)
	}
	if got := prepared.Resource().ProtoReflect().Get(resource.Descriptor().Fields().ByName("secret")).String(); got != "one-shot" {
		t.Fatalf("filtered secret = %q, want one-shot", got)
	}
}

func newLifecyclePlan(t *testing.T) (*ResourcePlan[proto.Message], *dynamicpb.Message) {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    stringPointer("lifecycle_internal_test.proto"),
		Package: stringPointer("lifecycletest"),
		Syntax:  stringPointer("proto2"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPointer("Profile"),
				Field: []*descriptorpb.FieldDescriptorProto{
					annotatedFieldDescriptor("bio", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL),
					annotatedFieldDescriptor("locale", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_REQUIRED),
					annotatedFieldDescriptor("origin", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL, annotations.FieldBehavior_IMMUTABLE),
				},
			},
			{
				Name: stringPointer("Resource"),
				Field: []*descriptorpb.FieldDescriptorProto{
					annotatedFieldDescriptor("profile", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".lifecycletest.Profile", annotations.FieldBehavior_OPTIONAL),
					annotatedFieldDescriptor("secret", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OPTIONAL, annotations.FieldBehavior_INPUT_ONLY),
					annotatedFieldDescriptor("server_value", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", annotations.FieldBehavior_OUTPUT_ONLY),
				},
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	descriptor := file.Messages().ByName("Resource")
	fields := make(map[string]FieldPlan)
	var identifiers []protoreflect.FieldDescriptor
	scanFields(fields, descriptor, "", map[protoreflect.FullName]bool{}, &identifiers)
	return &ResourcePlan[proto.Message]{descriptor: descriptor, fields: fields}, dynamicpb.NewMessage(descriptor)
}

func annotatedFieldDescriptor(
	name string,
	number int32,
	fieldType descriptorpb.FieldDescriptorProto_Type,
	typeName string,
	behaviors ...annotations.FieldBehavior,
) *descriptorpb.FieldDescriptorProto {
	field := fieldDescriptor(name, number, fieldType, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, typeName)
	field.Options = new(descriptorpb.FieldOptions)
	proto.SetExtension(field.Options, annotations.E_FieldBehavior, behaviors)
	return field
}

func setLifecycleString(message *dynamicpb.Message, path, value string) {
	parts := strings.Split(path, ".")
	current := message.ProtoReflect()
	for index, part := range parts {
		field := current.Descriptor().Fields().ByName(protoreflect.Name(part))
		if index == len(parts)-1 {
			current.Set(field, protoreflect.ValueOfString(value))
			return
		}
		current = current.Mutable(field).Message()
	}
}

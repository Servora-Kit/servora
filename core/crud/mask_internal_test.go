package crud

import (
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestNormalizeWriteMaskRejectsAncestorOverlapAndElementTraversal(t *testing.T) {
	t.Parallel()

	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:    stringPointer("mask_internal_test.proto"),
		Package: stringPointer("masktest"),
		Syntax:  stringPointer("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: stringPointer("Profile"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldDescriptor("bio", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ""),
				},
			},
			{
				Name: stringPointer("Resource"),
				Field: []*descriptorpb.FieldDescriptorProto{
					fieldDescriptor("profile", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, ".masktest.Profile"),
					fieldDescriptor("tags", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""),
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
	plan := &ResourcePlan[proto.Message]{descriptor: descriptor, fields: fields}
	resource := dynamicpb.NewMessage(descriptor)

	for _, paths := range [][]string{
		{"profile", "profile.bio"},
		{"tags.value"},
		{"tags[0]"},
	} {
		_, err := plan.NormalizeWriteMask(resource, &fieldmaskpb.FieldMask{Paths: paths})
		if !crudpb.IsCrudErrorReasonInvalidFieldMask(err) {
			t.Fatalf("NormalizeWriteMask(%v) error = %v, want INVALID_FIELD_MASK", paths, err)
		}
	}
}

package plugin

import (
	"sort"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/codegen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type messageGenerator struct {
	pkg     protoreflect.FullName
	message protoreflect.MessageDescriptor
}

func (m messageGenerator) Generate(f *codegen.File) {
	commentGenerator{descriptor: m.message}.generateLeading(f, 0)
	f.P("export type ", scopedDescriptorTypeName(m.pkg, m.message), " = {")
	fields := make([]protoreflect.FieldDescriptor, 0, m.message.Fields().Len())
	rangeFields(m.message, func(field protoreflect.FieldDescriptor) {
		fields = append(fields, field)
	})
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].JSONName() < fields[j].JSONName()
	})
	for _, field := range fields {
		commentGenerator{descriptor: field}.generateLeading(f, 1)
		fieldType := typeFromField(m.pkg, field)
		f.P(t(1), field.JSONName(), "?: ", fieldType.Reference(), ";")
	}

	f.P("};")
	f.P()
}

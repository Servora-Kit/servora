package plugin

import (
	"sort"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/codegen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type enumGenerator struct {
	pkg  protoreflect.FullName
	enum protoreflect.EnumDescriptor
}

func (e enumGenerator) Generate(f *codegen.File) {
	commentGenerator{descriptor: e.enum}.generateLeading(f, 0)
	f.P("export type ", scopedDescriptorTypeName(e.pkg, e.enum), " =")
	if e.enum.Values().Len() == 0 {
		Warn("enum %s has no values; generating fallback 'unknown' type", e.enum.FullName())
		f.P(t(1), "unknown;")
		return
	}
	values := make([]protoreflect.EnumValueDescriptor, 0, e.enum.Values().Len())
	rangeEnumValues(e.enum, func(value protoreflect.EnumValueDescriptor, _ bool) {
		values = append(values, value)
	})
	sort.Slice(values, func(i, j int) bool {
		return localeCompare(string(values[i].Name()), string(values[j].Name()))
	})
	if len(values) == 1 {
		commentGenerator{descriptor: values[0]}.generateLeading(f, 1)
		f.P(t(1), "'", string(values[0].Name()), "';")
		return
	}
	for i, value := range values {
		commentGenerator{descriptor: value}.generateLeading(f, 1)
		if i == len(values)-1 {
			f.P(t(1), "| '", string(value.Name()), "';")
		} else {
			f.P(t(1), "| '", string(value.Name()), "'")
		}
	}
}

package crud

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// WriteMask is an immutable normalized update path selection.
type WriteMask struct {
	paths    []string
	fields   []FieldPlan
	implicit bool
	wildcard bool
}

// Paths returns canonical sorted paths. A wildcard mask returns ["*"].
func (mask WriteMask) Paths() []string { return slices.Clone(mask.paths) }

// Implicit reports whether update_mask was omitted and derived from resource presence.
func (mask WriteMask) Implicit() bool { return mask.implicit }

// Wildcard reports whether the sole explicit path was "*".
func (mask WriteMask) Wildcard() bool { return mask.wildcard }

// Fields returns descriptor-resolved selected fields. Wildcard expansion occurs during lifecycle planning.
func (mask WriteMask) Fields() []FieldPlan { return slices.Clone(mask.fields) }

// NormalizeWriteMask validates explicit FieldMask paths or derives an implicit top-level mask.
func (plan *ResourcePlan[R]) NormalizeWriteMask(resource R, mask *fieldmaskpb.FieldMask) (WriteMask, error) {
	if isNilInterface(resource) {
		return WriteMask{}, invalidFieldMask("update_mask", "resource is nil")
	}
	if mask == nil {
		return plan.implicitWriteMask(resource), nil
	}
	return plan.explicitWriteMask(mask)
}

func (plan *ResourcePlan[R]) implicitWriteMask(resource proto.Message) WriteMask {
	message := resource.ProtoReflect()
	fields := plan.descriptor.Fields()
	paths := make([]string, 0, fields.Len())
	selected := make([]FieldPlan, 0, fields.Len())
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		if !isImplicitlySelected(message, field) {
			continue
		}
		path := string(field.Name())
		fieldPlan, ok := plan.fields[path]
		if !ok {
			continue
		}
		paths = append(paths, path)
		selected = append(selected, fieldPlan)
	}
	sortMaskPaths(paths, selected)
	return WriteMask{paths: paths, fields: selected, implicit: true}
}

func (plan *ResourcePlan[R]) explicitWriteMask(mask *fieldmaskpb.FieldMask) (WriteMask, error) {
	if len(mask.GetPaths()) == 1 && mask.GetPaths()[0] == "*" {
		return WriteMask{paths: []string{"*"}, wildcard: true}, nil
	}
	seen := make(map[string]FieldPlan, len(mask.GetPaths()))
	for _, path := range mask.GetPaths() {
		if path == "*" {
			return WriteMask{}, invalidFieldMask("update_mask", "* must be the only path")
		}
		if err := validateMaskPathSyntax(path); err != nil {
			return WriteMask{}, invalidFieldMask("update_mask", "%v", err)
		}
		field, ok := plan.fields[path]
		if !ok {
			return WriteMask{}, invalidFieldMask("update_mask", "unknown or unsupported path %q", path)
		}
		seen[path] = field
	}
	paths := make([]string, 0, len(seen))
	fields := make([]FieldPlan, 0, len(seen))
	for path, field := range seen {
		paths = append(paths, path)
		fields = append(fields, field)
	}
	sortMaskPaths(paths, fields)
	for index := 1; index < len(paths); index++ {
		if strings.HasPrefix(paths[index], paths[index-1]+".") {
			return WriteMask{}, invalidFieldMask(
				"update_mask",
				"ancestor path %q overlaps descendant %q",
				paths[index-1],
				paths[index],
			)
		}
	}
	return WriteMask{paths: paths, fields: fields}, nil
}

func validateMaskPathSyntax(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.TrimSpace(path) != path {
		return fmt.Errorf("path %q contains surrounding whitespace", path)
	}
	if strings.ContainsAny(path, "[]*") {
		return fmt.Errorf("path %q contains unsupported element traversal", path)
	}
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return fmt.Errorf("path %q contains an empty segment", path)
		}
	}
	return nil
}

func isImplicitlySelected(message protoreflect.Message, field protoreflect.FieldDescriptor) bool {
	if field.IsList() {
		return message.Get(field).List().Len() > 0
	}
	if field.IsMap() {
		return message.Get(field).Map().Len() > 0
	}
	if field.HasPresence() {
		return message.Has(field)
	}
	value := message.Get(field)
	switch field.Kind() {
	case protoreflect.BoolKind:
		return value.Bool()
	case protoreflect.EnumKind:
		return value.Enum() != field.Default().Enum()
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return value.Int() != 0
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return value.Uint() != 0
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return value.Float() != 0
	case protoreflect.StringKind:
		return value.String() != ""
	case protoreflect.BytesKind:
		return len(value.Bytes()) != 0
	default:
		return false
	}
}

func sortMaskPaths(paths []string, fields []FieldPlan) {
	indices := make([]int, len(paths))
	for index := range indices {
		indices[index] = index
	}
	sort.Slice(indices, func(left, right int) bool { return paths[indices[left]] < paths[indices[right]] })
	sortedPaths := make([]string, len(paths))
	sortedFields := make([]FieldPlan, len(fields))
	for target, source := range indices {
		sortedPaths[target] = paths[source]
		sortedFields[target] = fields[source]
	}
	copy(paths, sortedPaths)
	copy(fields, sortedFields)
}

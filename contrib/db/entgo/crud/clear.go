package crud

import (
	"fmt"
	"reflect"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ClearMutation is the generated Ent mutation surface used by same-name nullable clears.
type ClearMutation interface {
	ClearField(string) error
}

// ClearBinding configures one non-default masked-absent storage representation.
type ClearBinding[M ClearMutation] struct {
	path       string
	descriptor protoreflect.FieldDescriptor
	action     func(M) error
	err        error
}

// RenameClear maps one resource field's Clear intent to a differently named nullable Ent field.
func RenameClear[M ClearMutation](path FieldPath, backendField string) ClearBinding[M] {
	binding := ClearBinding[M]{path: fieldPathString(path), descriptor: fieldPathDescriptor(path)}
	if strings.TrimSpace(backendField) == "" {
		binding.err = fmt.Errorf("backend field is empty")
		return binding
	}
	binding.action = func(mutation M) error { return mutation.ClearField(backendField) }
	return binding
}

// ClearToValue maps one resource field's Clear intent to repository-owned mutation logic.
func ClearToValue[M ClearMutation](path FieldPath, action func(M) error) ClearBinding[M] {
	binding := ClearBinding[M]{path: fieldPathString(path), descriptor: fieldPathDescriptor(path), action: action}
	if action == nil {
		binding.err = fmt.Errorf("clear-to-value action is nil")
	}
	return binding
}

// ClearHelper is an immutable masked-absent Clear contract for one generated Ent mutation type.
type ClearHelper[M ClearMutation] struct {
	overrides map[string]func(M) error
}

// NewClearHelper validates and freezes rename and clear-to-value overrides.
func NewClearHelper[M ClearMutation](bindings ...ClearBinding[M]) (*ClearHelper[M], error) {
	helper := &ClearHelper[M]{overrides: make(map[string]func(M) error, len(bindings))}
	for index, binding := range bindings {
		if binding.err != nil {
			return nil, fmt.Errorf("entcrud: clear binding %d: %w", index, binding.err)
		}
		if strings.TrimSpace(binding.path) == "" || binding.descriptor == nil {
			return nil, fmt.Errorf("entcrud: clear binding %d resource field is invalid", index)
		}
		if binding.action == nil {
			return nil, fmt.Errorf("entcrud: clear binding %d action is nil", index)
		}
		if _, duplicate := helper.overrides[binding.path]; duplicate {
			return nil, fmt.Errorf("entcrud: clear field %q is configured more than once", binding.path)
		}
		helper.overrides[binding.path] = binding.action
	}
	return helper, nil
}

// Apply executes Clear intents from a normalized mutable leaf mask before Ent Save.
// Present values and repeated/map Replace(empty) intents are left to repository setters.
func (helper *ClearHelper[M]) Apply(
	resource proto.Message,
	mask *fieldmaskpb.FieldMask,
	mutation M,
) error {
	if helper == nil {
		return fmt.Errorf("entcrud: ClearHelper is nil")
	}
	if isNilProtoMessage(resource) {
		return fmt.Errorf("entcrud: clear resource is nil")
	}
	if mask == nil {
		return fmt.Errorf("entcrud: normalized write mask is nil")
	}
	if isNilGeneric(mutation) {
		return fmt.Errorf("entcrud: Ent mutation is nil")
	}

	message := resource.ProtoReflect()
	seen := make(map[string]struct{}, len(mask.GetPaths()))
	for _, path := range mask.GetPaths() {
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		if path == "*" {
			return fmt.Errorf("entcrud: normalized write mask still contains wildcard")
		}
		leaf, present, err := clearPathPresence(message, path)
		if err != nil {
			return fmt.Errorf("entcrud: clear path %q: %w", path, err)
		}
		if leaf.IsList() || leaf.IsMap() {
			continue
		}
		if !leaf.HasPresence() {
			return fmt.Errorf("entcrud: clear path %q has no Proto presence", path)
		}
		if present {
			continue
		}
		if action, ok := helper.overrides[path]; ok {
			if err := action(mutation); err != nil {
				return fmt.Errorf("entcrud: clear path %q override: %w", path, err)
			}
			continue
		}
		if strings.Contains(path, ".") {
			return fmt.Errorf("entcrud: clear path %q requires an explicit nested override", path)
		}
		if err := mutation.ClearField(path); err != nil {
			return fmt.Errorf("entcrud: clear path %q as same-name Ent field: %w", path, err)
		}
	}
	return nil
}

func clearPathPresence(message protoreflect.Message, path string) (protoreflect.FieldDescriptor, bool, error) {
	parts := strings.Split(path, ".")
	current := message
	for index, part := range parts {
		field := current.Descriptor().Fields().ByName(protoreflect.Name(part))
		if field == nil {
			return nil, false, fmt.Errorf("field is unknown")
		}
		if index == len(parts)-1 {
			return field, current.Has(field), nil
		}
		if field.IsList() || field.IsMap() || field.Message() == nil {
			return nil, false, fmt.Errorf("intermediate field %q is not a singular message", part)
		}
		if !current.Has(field) {
			return resolveClearLeaf(field.Message(), parts[index+1:])
		}
		current = current.Get(field).Message()
	}
	return nil, false, fmt.Errorf("path is empty")
}

func resolveClearLeaf(
	descriptor protoreflect.MessageDescriptor,
	parts []string,
) (protoreflect.FieldDescriptor, bool, error) {
	for index, part := range parts {
		field := descriptor.Fields().ByName(protoreflect.Name(part))
		if field == nil {
			return nil, false, fmt.Errorf("field is unknown")
		}
		if index == len(parts)-1 {
			return field, false, nil
		}
		if field.IsList() || field.IsMap() || field.Message() == nil {
			return nil, false, fmt.Errorf("intermediate field %q is not a singular message", part)
		}
		descriptor = field.Message()
	}
	return nil, false, fmt.Errorf("path is empty")
}

func isNilProtoMessage(message proto.Message) bool {
	if message == nil {
		return true
	}
	value := reflect.ValueOf(message)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

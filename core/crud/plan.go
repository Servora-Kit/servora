// Package crud implements backend-neutral CRUD request preparation.
package crud

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ResourcePlan is the immutable runtime description of one AIP resource.
type ResourcePlan[R proto.Message] struct {
	descriptor     protoreflect.MessageDescriptor
	resourceType   string
	patterns       []string
	nameMatcher    *ResourceNameMatcher
	identifier     protoreflect.FieldDescriptor
	fields         map[string]FieldPlan
	writablePaths  []string
	queryablePaths []string
}

// FieldPlan describes one canonical field path in a resource plan.
type FieldPlan struct {
	path       string
	descriptor protoreflect.FieldDescriptor
	behaviors  map[annotations.FieldBehavior]struct{}
}

// MustBuildResourcePlan builds an immutable resource plan or panics on an
// invalid descriptor or generic resource type.
func MustBuildResourcePlan[R proto.Message](descriptor protoreflect.MessageDescriptor) *ResourcePlan[R] {
	plan, err := buildResourcePlan[R](descriptor)
	if err != nil {
		panic(err)
	}
	return plan
}

// Descriptor returns the resource message descriptor.
func (p *ResourcePlan[R]) Descriptor() protoreflect.MessageDescriptor {
	return p.descriptor
}

// ResourceType returns the google.api.resource type.
func (p *ResourcePlan[R]) ResourceType() string {
	return p.resourceType
}

// Patterns returns a copy of the declared google.api.resource patterns.
func (p *ResourcePlan[R]) Patterns() []string {
	return slices.Clone(p.patterns)
}

// ParseName validates a canonical resource name against the plan patterns.
func (p *ResourcePlan[R]) ParseName(value string) (ResourceName, error) {
	name, err := p.nameMatcher.Parse(value)
	if err != nil {
		return ResourceName{}, invalidResourceName("name", "%v", err)
	}
	return name, nil
}

// Identifier returns the unique IDENTIFIER field descriptor.
func (p *ResourcePlan[R]) Identifier() protoreflect.FieldDescriptor {
	return p.identifier
}

// Field returns the plan for a canonical protobuf snake_case field path.
func (p *ResourcePlan[R]) Field(path string) (FieldPlan, bool) {
	field, ok := p.fields[path]
	return field, ok
}

// WritablePaths returns mutable client-input paths in canonical sorted order.
func (p *ResourcePlan[R]) WritablePaths() []string {
	return slices.Clone(p.writablePaths)
}

// QueryablePaths returns non-identifier, non-input-only paths in canonical sorted order.
func (p *ResourcePlan[R]) QueryablePaths() []string {
	return slices.Clone(p.queryablePaths)
}

// Path returns the canonical protobuf snake_case path.
func (f FieldPlan) Path() string {
	return f.path
}

// Descriptor returns the leaf field descriptor.
func (f FieldPlan) Descriptor() protoreflect.FieldDescriptor {
	return f.descriptor
}

// HasBehavior reports whether the field declares the provided AIP behavior.
func (f FieldPlan) HasBehavior(behavior annotations.FieldBehavior) bool {
	_, ok := f.behaviors[behavior]
	return ok
}

func buildResourcePlan[R proto.Message](descriptor protoreflect.MessageDescriptor) (*ResourcePlan[R], error) {
	if descriptor == nil {
		return nil, fmt.Errorf("crud: resource descriptor is nil")
	}
	if err := validateResourceType[R](descriptor); err != nil {
		return nil, err
	}

	resource, ok := proto.GetExtension(descriptor.Options(), annotations.E_Resource).(*annotations.ResourceDescriptor)
	if !ok || resource == nil {
		return nil, fmt.Errorf("crud: %s has no google.api.resource annotation", descriptor.FullName())
	}
	if resource.GetType() == "" {
		return nil, fmt.Errorf("crud: %s resource type is empty", descriptor.FullName())
	}
	if len(resource.GetPattern()) == 0 {
		return nil, fmt.Errorf("crud: %s has no resource pattern", descriptor.FullName())
	}
	nameMatcher, err := NewResourceNameMatcher(resource.GetPattern()...)
	if err != nil {
		return nil, fmt.Errorf("crud: %s resource patterns: %w", descriptor.FullName(), err)
	}

	plan := &ResourcePlan[R]{
		descriptor:   descriptor,
		resourceType: resource.GetType(),
		patterns:     slices.Clone(resource.GetPattern()),
		nameMatcher:  nameMatcher,
		fields:       make(map[string]FieldPlan),
	}

	var identifiers []protoreflect.FieldDescriptor
	scanFields(plan.fields, descriptor, "", map[protoreflect.FullName]bool{}, &identifiers)
	if len(identifiers) != 1 {
		return nil, fmt.Errorf("crud: %s must have exactly one IDENTIFIER field, got %d", descriptor.FullName(), len(identifiers))
	}
	identifier := identifiers[0]
	if identifier.Kind() != protoreflect.StringKind || identifier.IsList() || identifier.IsMap() {
		return nil, fmt.Errorf("crud: %s IDENTIFIER field must be a singular string", identifier.FullName())
	}
	plan.identifier = identifier

	plan.writablePaths = make([]string, 0, len(plan.fields))
	plan.queryablePaths = make([]string, 0, len(plan.fields))
	for path, field := range plan.fields {
		if isWritableField(field) {
			plan.writablePaths = append(plan.writablePaths, path)
		}
		if isQueryableField(field) {
			plan.queryablePaths = append(plan.queryablePaths, path)
		}
	}
	sort.Strings(plan.writablePaths)
	sort.Strings(plan.queryablePaths)

	return plan, nil
}

func validateResourceType[R proto.Message](descriptor protoreflect.MessageDescriptor) error {
	typeOf := reflect.TypeFor[R]()
	if typeOf.Kind() != reflect.Pointer {
		return fmt.Errorf("crud: resource type %s must be a protobuf message pointer", typeOf)
	}
	value := reflect.New(typeOf.Elem()).Interface()
	message, ok := value.(proto.Message)
	if !ok {
		return fmt.Errorf("crud: resource type %s does not implement proto.Message", typeOf)
	}
	actual := message.ProtoReflect().Descriptor()
	if actual.FullName() != descriptor.FullName() {
		return fmt.Errorf("crud: resource type %s does not match descriptor %s", actual.FullName(), descriptor.FullName())
	}
	return nil
}

func scanFields(
	result map[string]FieldPlan,
	descriptor protoreflect.MessageDescriptor,
	prefix string,
	ancestors map[protoreflect.FullName]bool,
	identifiers *[]protoreflect.FieldDescriptor,
) {
	if ancestors[descriptor.FullName()] {
		return
	}
	nextAncestors := make(map[protoreflect.FullName]bool, len(ancestors)+1)
	for name := range ancestors {
		nextAncestors[name] = true
	}
	nextAncestors[descriptor.FullName()] = true

	fields := descriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		descriptorField := fields.Get(index)
		path := string(descriptorField.Name())
		if prefix != "" {
			path = prefix + "." + path
		}
		field := FieldPlan{
			path:       path,
			descriptor: descriptorField,
			behaviors:  fieldBehaviors(descriptorField),
		}
		result[path] = field
		if field.HasBehavior(annotations.FieldBehavior_IDENTIFIER) {
			*identifiers = append(*identifiers, descriptorField)
		}

		if shouldTraverse(descriptorField) {
			scanFields(result, descriptorField.Message(), path, nextAncestors, identifiers)
		}
	}
}

func fieldBehaviors(field protoreflect.FieldDescriptor) map[annotations.FieldBehavior]struct{} {
	values, _ := proto.GetExtension(field.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
	result := make(map[annotations.FieldBehavior]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func shouldTraverse(field protoreflect.FieldDescriptor) bool {
	if field.Kind() != protoreflect.MessageKind || field.IsList() || field.IsMap() {
		return false
	}
	return !strings.HasPrefix(string(field.Message().FullName()), "google.protobuf.")
}

func isWritableField(field FieldPlan) bool {
	if field.descriptor.Name() == "etag" {
		return false
	}
	return !field.HasBehavior(annotations.FieldBehavior_IDENTIFIER) &&
		!field.HasBehavior(annotations.FieldBehavior_OUTPUT_ONLY) &&
		!field.HasBehavior(annotations.FieldBehavior_IMMUTABLE)
}

func isQueryableField(field FieldPlan) bool {
	return !field.HasBehavior(annotations.FieldBehavior_IDENTIFIER) &&
		!field.HasBehavior(annotations.FieldBehavior_INPUT_ONLY)
}

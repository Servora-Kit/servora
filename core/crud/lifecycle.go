package crud

import (
	"bytes"
	"sort"
	"strings"

	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ListOptions contains normalized AIP list controls without business behavior.
type ListOptions struct {
	ShowDeleted bool
}

// UpdateOptions contains normalized AIP update controls without business behavior.
type UpdateOptions struct {
	AllowMissing bool
	Etag         string
}

// DeleteOptions contains normalized AIP delete controls without business behavior.
type DeleteOptions struct {
	AllowMissing bool
	Force        bool
	Etag         string
}

// PreparedCreate contains a filtered resource safe to pass beyond the service boundary.
type PreparedCreate[R proto.Message] struct {
	resource R
}

// Resource returns a clone of the filtered create resource.
func (prepared PreparedCreate[R]) Resource() R { return cloneResource(prepared.resource) }

// ImmutableComparison describes one direct or ancestor-selected immutable value comparison.
type ImmutableComparison struct {
	path   string
	direct bool
}

// Path returns the immutable resource field path.
func (comparison ImmutableComparison) Path() string { return comparison.path }

// Direct reports whether the immutable path itself was selected.
func (comparison ImmutableComparison) Direct() bool { return comparison.direct }

type immutableComparisonPlan struct {
	ImmutableComparison
	expectedPresent bool
}

// PreparedUpdate contains filtered mutable values, a leaf WriteMask, immutable comparisons and options.
type PreparedUpdate[R proto.Message] struct {
	resource    R
	writeMask   *fieldmaskpb.FieldMask
	comparisons []immutableComparisonPlan
	expected    R
	options     UpdateOptions
}

// Resource returns a clone containing only mutable values selected by WriteMask.
func (prepared PreparedUpdate[R]) Resource() R { return cloneResource(prepared.resource) }

// WriteMask returns a clone of the normalized mutable leaf mask.
func (prepared PreparedUpdate[R]) WriteMask() *fieldmaskpb.FieldMask {
	return proto.CloneOf(prepared.writeMask)
}

// ImmutableComparisons returns immutable compare metadata without mutable value aliases.
func (prepared PreparedUpdate[R]) ImmutableComparisons() []ImmutableComparison {
	result := make([]ImmutableComparison, len(prepared.comparisons))
	for index, comparison := range prepared.comparisons {
		result[index] = comparison.ImmutableComparison
	}
	return result
}

// Options returns normalized AIP options. Their business semantics remain in biz.
func (prepared PreparedUpdate[R]) Options() UpdateOptions { return prepared.options }

// PrepareCreate validates REQUIRED input and removes IDENTIFIER/OUTPUT_ONLY fields.
func (plan *ResourcePlan[R]) PrepareCreate(resource R) (PreparedCreate[R], error) {
	if isNilInterface(resource) {
		return PreparedCreate[R]{}, invalidFieldValue("resource", "is nil")
	}
	if err := plan.validateCreateRequired(resource.ProtoReflect(), plan.descriptor, ""); err != nil {
		return PreparedCreate[R]{}, err
	}
	filtered := cloneResource(resource)
	plan.clearCreateSystemFields(filtered.ProtoReflect(), plan.descriptor, "")
	return PreparedCreate[R]{resource: filtered}, nil
}

// PrepareUpdate normalizes FieldMask, lifecycle partitions and immutable compare intents.
func (plan *ResourcePlan[R]) PrepareUpdate(
	resource R,
	mask *fieldmaskpb.FieldMask,
	options UpdateOptions,
) (PreparedUpdate[R], error) {
	if isNilInterface(resource) {
		return PreparedUpdate[R]{}, invalidFieldValue("resource", "is nil")
	}
	normalized, err := plan.NormalizeWriteMask(resource, mask)
	if err != nil {
		return PreparedUpdate[R]{}, err
	}
	request := resource.ProtoReflect()
	writeSet := make(map[string]struct{})
	comparisons := make([]immutableComparisonPlan, 0)

	if normalized.wildcard {
		fields := plan.descriptor.Fields()
		for index := 0; index < fields.Len(); index++ {
			field := fields.Get(index)
			if err := plan.collectUpdateSelection(
				request,
				string(field.Name()),
				field,
				false,
				writeSet,
				&comparisons,
			); err != nil {
				return PreparedUpdate[R]{}, err
			}
		}
	} else {
		for _, path := range normalized.paths {
			field := plan.fields[path].descriptor
			if err := plan.collectUpdateSelection(request, path, field, true, writeSet, &comparisons); err != nil {
				return PreparedUpdate[R]{}, err
			}
		}
	}

	writePaths := make([]string, 0, len(writeSet))
	for path := range writeSet {
		writePaths = append(writePaths, path)
	}
	sort.Strings(writePaths)
	sort.Slice(comparisons, func(left, right int) bool {
		return comparisons[left].path < comparisons[right].path
	})
	filtered := cloneResource(resource)
	plan.filterUpdateResource(filtered.ProtoReflect(), writeSet)
	return PreparedUpdate[R]{
		resource:    filtered,
		writeMask:   &fieldmaskpb.FieldMask{Paths: writePaths},
		comparisons: comparisons,
		expected:    cloneResource(resource),
		options:     options,
	}, nil
}

// ValidateImmutable compares prepared immutable intentions against the current resource.
func (prepared PreparedUpdate[R]) ValidateImmutable(current R) error {
	if isNilInterface(current) {
		return internalError("resource", "current immutable comparison resource is nil")
	}
	expected := prepared.expected.ProtoReflect()
	actual := current.ProtoReflect()
	for _, comparison := range prepared.comparisons {
		field := fieldAtPath(expected.Descriptor(), comparison.path)
		if field == nil {
			return internalError(comparison.path, "immutable field descriptor is missing")
		}
		actualPresent := pathPresent(actual, comparison.path, comparison.direct)
		if actualPresent != comparison.expectedPresent {
			return invalidFieldValue(comparison.path, "immutable presence cannot change")
		}
		if !comparison.expectedPresent {
			continue
		}
		expectedValue, ok := valueAtPath(expected, comparison.path)
		if !ok {
			return internalError(comparison.path, "expected immutable value is unavailable")
		}
		actualValue, ok := valueAtPath(actual, comparison.path)
		if !ok || !fieldValuesEqual(field, expectedValue, actualValue) {
			return invalidFieldValue(comparison.path, "immutable value cannot change")
		}
	}
	return nil
}

func (plan *ResourcePlan[R]) collectUpdateSelection(
	request protoreflect.Message,
	path string,
	field protoreflect.FieldDescriptor,
	direct bool,
	writeSet map[string]struct{},
	comparisons *[]immutableComparisonPlan,
) error {
	classification := plan.classifyLifecycle(path)
	if classification.system {
		return nil
	}
	if classification.immutable {
		present := pathPresent(request, path, direct)
		if direct || present {
			*comparisons = append(*comparisons, immutableComparisonPlan{
				ImmutableComparison: ImmutableComparison{path: path, direct: direct},
				expectedPresent:     present,
			})
		}
		return nil
	}
	if descriptorHasBehavior(field, annotations.FieldBehavior_REQUIRED) && !pathTruthy(request, path) {
		return invalidFieldValue(path, "REQUIRED field must be present and truthy")
	}
	if shouldTraverse(field) {
		children := field.Message().Fields()
		for index := 0; index < children.Len(); index++ {
			child := children.Get(index)
			childPath := path + "." + string(child.Name())
			if err := plan.collectUpdateSelection(request, childPath, child, false, writeSet, comparisons); err != nil {
				return err
			}
		}
		return nil
	}
	writeSet[path] = struct{}{}
	return nil
}

type lifecycleClassification struct {
	system    bool
	immutable bool
}

func (plan *ResourcePlan[R]) classifyLifecycle(path string) lifecycleClassification {
	parts := strings.Split(path, ".")
	for index := range parts {
		prefix := strings.Join(parts[:index+1], ".")
		field, ok := plan.fields[prefix]
		if !ok {
			continue
		}
		if field.HasBehavior(annotations.FieldBehavior_IDENTIFIER) || field.HasBehavior(annotations.FieldBehavior_OUTPUT_ONLY) {
			return lifecycleClassification{system: true}
		}
		if field.HasBehavior(annotations.FieldBehavior_IMMUTABLE) {
			return lifecycleClassification{immutable: true}
		}
	}
	return lifecycleClassification{}
}

func (plan *ResourcePlan[R]) validateCreateRequired(
	message protoreflect.Message,
	descriptor protoreflect.MessageDescriptor,
	prefix string,
) error {
	fields := descriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		path := string(field.Name())
		if prefix != "" {
			path = prefix + "." + path
		}
		if plan.classifyLifecycle(path).system {
			continue
		}
		if descriptorHasBehavior(field, annotations.FieldBehavior_REQUIRED) && !fieldTruthy(message, field) {
			return invalidFieldValue(path, "REQUIRED field must be present and truthy")
		}
		if !shouldTraverse(field) || !message.Has(field) {
			continue
		}
		if err := plan.validateCreateRequired(message.Get(field).Message(), field.Message(), path); err != nil {
			return err
		}
	}
	return nil
}

func (plan *ResourcePlan[R]) clearCreateSystemFields(
	message protoreflect.Message,
	descriptor protoreflect.MessageDescriptor,
	prefix string,
) {
	fields := descriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		path := string(field.Name())
		if prefix != "" {
			path = prefix + "." + path
		}
		if plan.classifyLifecycle(path).system {
			message.Clear(field)
			continue
		}
		if shouldTraverse(field) && message.Has(field) {
			plan.clearCreateSystemFields(message.Mutable(field).Message(), field.Message(), path)
		}
	}
}

func (plan *ResourcePlan[R]) filterUpdateResource(message protoreflect.Message, writeSet map[string]struct{}) {
	for path, field := range plan.fields {
		classification := plan.classifyLifecycle(path)
		if shouldTraverse(field.descriptor) && !classification.system && !classification.immutable {
			continue
		}
		if _, selected := writeSet[path]; selected {
			continue
		}
		clearPath(message, path)
	}
}

func cloneResource[R proto.Message](resource R) R {
	return proto.Clone(resource).(R)
}

func fieldAtPath(descriptor protoreflect.MessageDescriptor, path string) protoreflect.FieldDescriptor {
	parts := strings.Split(path, ".")
	for index, segment := range parts {
		field := descriptor.Fields().ByName(protoreflect.Name(segment))
		if field == nil {
			return nil
		}
		if index == len(parts)-1 {
			return field
		}
		if field.Kind() != protoreflect.MessageKind {
			return nil
		}
		descriptor = field.Message()
	}
	return nil
}

func valueAtPath(message protoreflect.Message, path string) (protoreflect.Value, bool) {
	parts := strings.Split(path, ".")
	for index, segment := range parts {
		field := message.Descriptor().Fields().ByName(protoreflect.Name(segment))
		if field == nil {
			return protoreflect.Value{}, false
		}
		if index == len(parts)-1 {
			return message.Get(field), true
		}
		if field.HasPresence() && !message.Has(field) {
			return protoreflect.Value{}, false
		}
		message = message.Get(field).Message()
	}
	return protoreflect.Value{}, false
}

func pathPresent(message protoreflect.Message, path string, direct bool) bool {
	parts := strings.Split(path, ".")
	for index, segment := range parts {
		field := message.Descriptor().Fields().ByName(protoreflect.Name(segment))
		if field == nil {
			return false
		}
		if index == len(parts)-1 {
			if field.IsList() {
				return message.Get(field).List().Len() > 0
			}
			if field.IsMap() {
				return message.Get(field).Map().Len() > 0
			}
			if field.HasPresence() {
				return message.Has(field)
			}
			return direct || isImplicitlySelected(message, field)
		}
		if field.HasPresence() && !message.Has(field) {
			return false
		}
		message = message.Get(field).Message()
	}
	return false
}

func pathTruthy(message protoreflect.Message, path string) bool {
	parts := strings.Split(path, ".")
	for index, segment := range parts {
		field := message.Descriptor().Fields().ByName(protoreflect.Name(segment))
		if field == nil {
			return false
		}
		if index == len(parts)-1 {
			return fieldTruthy(message, field)
		}
		if field.HasPresence() && !message.Has(field) {
			return false
		}
		message = message.Get(field).Message()
	}
	return false
}

func fieldTruthy(message protoreflect.Message, field protoreflect.FieldDescriptor) bool {
	if field.IsList() {
		return message.Get(field).List().Len() > 0
	}
	if field.IsMap() {
		return message.Get(field).Map().Len() > 0
	}
	if field.HasPresence() && !message.Has(field) {
		return false
	}
	value := message.Get(field)
	if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
		nested := value.Message()
		fields := field.Message().Fields()
		for index := 0; index < fields.Len(); index++ {
			if fieldTruthy(nested, fields.Get(index)) {
				return true
			}
		}
		return false
	}
	return isImplicitlySelected(message, field)
}

func clearPath(message protoreflect.Message, path string) {
	parts := strings.Split(path, ".")
	for index, segment := range parts {
		field := message.Descriptor().Fields().ByName(protoreflect.Name(segment))
		if field == nil {
			return
		}
		if index == len(parts)-1 {
			message.Clear(field)
			return
		}
		if field.HasPresence() && !message.Has(field) {
			return
		}
		message = message.Mutable(field).Message()
	}
}

func descriptorHasBehavior(field protoreflect.FieldDescriptor, behavior annotations.FieldBehavior) bool {
	values, _ := proto.GetExtension(field.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
	for _, value := range values {
		if value == behavior {
			return true
		}
	}
	return false
}

func fieldValuesEqual(
	field protoreflect.FieldDescriptor,
	left protoreflect.Value,
	right protoreflect.Value,
) bool {
	if field.IsList() {
		leftList, rightList := left.List(), right.List()
		if leftList.Len() != rightList.Len() {
			return false
		}
		for index := range leftList.Len() {
			if !singularFieldValuesEqual(field, leftList.Get(index), rightList.Get(index)) {
				return false
			}
		}
		return true
	}
	if field.IsMap() {
		leftMap, rightMap := left.Map(), right.Map()
		if leftMap.Len() != rightMap.Len() {
			return false
		}
		equal := true
		leftMap.Range(func(key protoreflect.MapKey, leftValue protoreflect.Value) bool {
			if !rightMap.Has(key) || !singularFieldValuesEqual(field.MapValue(), leftValue, rightMap.Get(key)) {
				equal = false
				return false
			}
			return true
		})
		return equal
	}
	return singularFieldValuesEqual(field, left, right)
}

func singularFieldValuesEqual(
	field protoreflect.FieldDescriptor,
	left protoreflect.Value,
	right protoreflect.Value,
) bool {
	switch field.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return proto.Equal(left.Message().Interface(), right.Message().Interface())
	case protoreflect.BytesKind:
		return bytes.Equal(left.Bytes(), right.Bytes())
	default:
		return left.Interface() == right.Interface()
	}
}

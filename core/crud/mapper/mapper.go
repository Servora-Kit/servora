package mapper

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/jinzhu/copier"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FieldPath is implemented by generated typed resource field identities.
type FieldPath interface {
	String() string
	Descriptor() protoreflect.FieldDescriptor
}

// Option configures a ResourceMapper before immutable construction.
type Option interface {
	apply(*mapperOptions) error
}

type optionFunc func(*mapperOptions) error

func (option optionFunc) apply(options *mapperOptions) error { return option(options) }

type fieldMappingOption struct {
	poField string
	field   FieldPath
}

type hookOption struct {
	poType  reflect.Type
	dtoType reflect.Type
	invoke  func(any, proto.Message) error
}

type resourceNameOption struct {
	poType reflect.Type
	invoke func(any) (string, error)
}

type mapperOptions struct {
	converters   []copier.TypeConverter
	mappings     []fieldMappingOption
	flattens     []FieldPath
	hooks        []hookOption
	resourceName *resourceNameOption
}

// WithConverters appends explicit converters after the framework defaults.
func WithConverters(converters ...TypeConverter) Option {
	cloned := slices.Clone(converters)
	return optionFunc(func(options *mapperOptions) error {
		for index, converter := range cloned {
			if converter.Fn == nil || converter.SrcType == nil || converter.DstType == nil {
				return fmt.Errorf("mapper: converter %d is incomplete", index)
			}
		}
		options.converters = append(options.converters, cloned...)
		return nil
	})
}

// WithFieldMapping maps one PO Go field to one generated top-level or nested singular-message resource field.
func WithFieldMapping(poField string, resourceField FieldPath) Option {
	return optionFunc(func(options *mapperOptions) error {
		if strings.TrimSpace(poField) == "" {
			return fmt.Errorf("mapper: PO field name is empty")
		}
		if isNilFieldPath(resourceField) {
			return fmt.Errorf("mapper: resource field path is nil")
		}
		options.mappings = append(options.mappings, fieldMappingOption{poField: poField, field: resourceField})
		return nil
	})
}

// WithFlatten automatically projects exported same-name PO fields into the direct
// leaf fields of each specified singular nested message (e.g. Profile or Profile.Address).
// Explicit WithFieldMapping entries take precedence over flattened same-name matching.
func WithFlatten(subMessages ...FieldPath) Option {
	cloned := slices.Clone(subMessages)
	return optionFunc(func(options *mapperOptions) error {
		if len(cloned) == 0 {
			return fmt.Errorf("mapper: WithFlatten requires at least one sub-message field")
		}
		for index, sub := range cloned {
			if isNilFieldPath(sub) {
				return fmt.Errorf("mapper: WithFlatten field path %d is nil", index)
			}
		}
		options.flattens = append(options.flattens, cloned...)
		return nil
	})
}

// WithPostToDTOHook adds a typed deterministic local post-projection hook.
func WithPostToDTOHook[DTO proto.Message, PO any](hook func(*PO, DTO) error) Option {
	return optionFunc(func(options *mapperOptions) error {
		if hook == nil {
			return fmt.Errorf("mapper: post hook is nil")
		}
		options.hooks = append(options.hooks, hookOption{
			poType:  reflect.TypeFor[PO](),
			dtoType: reflect.TypeFor[DTO](),
			invoke: func(po any, dto proto.Message) error {
				return hook(po.(*PO), dto.(DTO))
			},
		})
		return nil
	})
}

// WithResourceName configures the canonical resource-name formatter.
func WithResourceName[PO any](formatter func(*PO) (string, error)) Option {
	return optionFunc(func(options *mapperOptions) error {
		if formatter == nil {
			return fmt.Errorf("mapper: resource name formatter is nil")
		}
		if options.resourceName != nil {
			return fmt.Errorf("mapper: resource name formatter is configured more than once")
		}
		options.resourceName = &resourceNameOption{
			poType: reflect.TypeFor[PO](),
			invoke: func(po any) (string, error) {
				return formatter(po.(*PO))
			},
		}
		return nil
	})
}

type nestedStep struct {
	fieldIndex []int
	elemType   reflect.Type
}

type nestedFieldProjection struct {
	sourceIndex []int
	steps       []nestedStep
	leafIndex   []int
	compute     func(reflect.Value) (reflect.Value, bool, error)
}

// ResourceMapper projects storage POs to generated CRUD resource messages.
type ResourceMapper[DTO proto.Message, PO any] struct {
	dtoType      reflect.Type
	poType       reflect.Type
	copyOption   copier.Option
	nested       []nestedFieldProjection
	hooks        []hookOption
	resourceName *resourceNameOption
	identifier   protoreflect.FieldDescriptor
}

// NewResourceMapper validates configuration and returns an immutable read mapper.
func NewResourceMapper[DTO proto.Message, PO any](options ...Option) (*ResourceMapper[DTO, PO], error) {
	dtoType := reflect.TypeFor[DTO]()
	if dtoType.Kind() != reflect.Pointer || !dtoType.Implements(reflect.TypeFor[proto.Message]()) {
		return nil, fmt.Errorf("mapper: DTO %v must be a protobuf message pointer", dtoType)
	}
	poType := reflect.TypeFor[PO]()
	if poType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("mapper: PO %v must be a non-pointer struct", poType)
	}
	configuration := mapperOptions{converters: builtinConverters()}
	for index, option := range options {
		if option == nil {
			return nil, fmt.Errorf("mapper: option %d is nil", index)
		}
		if err := option.apply(&configuration); err != nil {
			return nil, err
		}
	}
	converters, err := validateAndWrapConverters(configuration.converters)
	if err != nil {
		return nil, err
	}
	configuration.converters = converters
	dto := reflect.New(dtoType.Elem()).Interface().(DTO)
	descriptor := dto.ProtoReflect().Descriptor()
	if configuration.resourceName != nil && configuration.resourceName.poType != poType {
		return nil, fmt.Errorf("mapper: resource name formatter PO type %v, want %v", configuration.resourceName.poType, poType)
	}
	identifier, err := resourceIdentifier(descriptor)
	if err != nil {
		return nil, err
	}
	copyOption := copier.Option{
		IgnoreEmpty: false,
		DeepCopy:    true,
		Converters:  slices.Clone(configuration.converters),
	}
	fieldNameMap := make(map[string]string, len(configuration.mappings))
	explicitSources := make(map[string]struct{}, len(configuration.mappings))
	seenSources := make(map[string]struct{}, len(configuration.mappings))
	seenTargets := make(map[protoreflect.FullName]struct{}, len(configuration.mappings))
	seenTargetPaths := make(map[string]struct{}, len(configuration.mappings))
	nestedTopLevelTargets := make(map[protoreflect.FullName]struct{})
	var nested []nestedFieldProjection
	for _, mapping := range configuration.mappings {
		source, ok := poType.FieldByName(mapping.poField)
		if !ok || source.PkgPath != "" {
			return nil, fmt.Errorf("mapper: PO field %q does not exist or is unexported", mapping.poField)
		}
		if _, duplicate := seenSources[mapping.poField]; duplicate {
			return nil, fmt.Errorf("mapper: PO field %q is mapped more than once", mapping.poField)
		}
		target := mapping.field.Descriptor()
		rawPath := mapping.field.String()
		if target == nil || strings.TrimSpace(rawPath) == "" {
			return nil, fmt.Errorf("mapper: resource field %q has invalid descriptor", rawPath)
		}
		parts := strings.Split(rawPath, ".")
		if slices.Contains(parts, "") {
			return nil, fmt.Errorf("mapper: resource field path %q has an empty segment", rawPath)
		}
		for existingPath := range seenTargetPaths {
			if existingPath == rawPath {
				return nil, fmt.Errorf("mapper: resource field %s is mapped more than once", target.FullName())
			}
			if strings.HasPrefix(rawPath, existingPath+".") || strings.HasPrefix(existingPath, rawPath+".") {
				return nil, fmt.Errorf("mapper: resource field %q overlaps with mapped path %q", rawPath, existingPath)
			}
		}
		if _, duplicate := seenTargets[target.FullName()]; duplicate {
			return nil, fmt.Errorf("mapper: resource field %s is mapped more than once", target.FullName())
		}
		if len(parts) == 1 {
			if target.ContainingMessage().FullName() != descriptor.FullName() {
				return nil, fmt.Errorf("mapper: resource field %q is not a top-level field of %s", rawPath, descriptor.FullName())
			}
			if rawPath != string(target.Name()) {
				return nil, fmt.Errorf("mapper: resource field path %q does not match descriptor %s", rawPath, target.Name())
			}
			destination, ok := protoGoField(dtoType.Elem(), target.Name())
			if !ok {
				return nil, fmt.Errorf("mapper: generated DTO Go field for %s is missing", target.FullName())
			}
			if !typesMappable(source.Type, destination.Type, configuration.converters) {
				return nil, fmt.Errorf("mapper: no converter from %v to %v for %s", source.Type, destination.Type, target.FullName())
			}
			fieldNameMap[mapping.poField] = destination.Name
		} else {
			projection, rootField, err := compileNestedProjection(
				descriptor,
				dtoType.Elem(),
				source,
				rawPath,
				parts,
				target,
				configuration.converters,
			)
			if err != nil {
				return nil, err
			}
			nested = append(nested, projection)
			nestedTopLevelTargets[rootField.FullName()] = struct{}{}
			fieldNameMap[mapping.poField] = ""
		}
		explicitSources[mapping.poField] = struct{}{}
		seenSources[mapping.poField] = struct{}{}
		seenTargets[target.FullName()] = struct{}{}
		seenTargetPaths[rawPath] = struct{}{}
	}
	if len(configuration.flattens) > 0 {
		if err := expandFlattenedProjections(
			descriptor,
			dtoType.Elem(),
			poType,
			identifier,
			configuration.resourceName != nil,
			configuration.flattens,
			configuration.converters,
			explicitSources,
			seenSources,
			seenTargets,
			seenTargetPaths,
			nestedTopLevelTargets,
			fieldNameMap,
			&nested,
		); err != nil {
			return nil, err
		}
	}
	fields := descriptor.Fields()
	for index := range fields.Len() {
		target := fields.Get(index)
		if _, explicitlyMapped := seenTargets[target.FullName()]; explicitlyMapped {
			continue
		}
		destination, ok := protoGoField(dtoType.Elem(), target.Name())
		if !ok {
			return nil, fmt.Errorf("mapper: generated DTO Go field for %s is missing", target.FullName())
		}
		source, ok := poType.FieldByName(destination.Name)
		if !ok || source.PkgPath != "" {
			continue
		}
		if _, explicitlyMapped := seenSources[source.Name]; explicitlyMapped {
			continue
		}
		if _, hasNestedChildren := nestedTopLevelTargets[target.FullName()]; hasNestedChildren {
			return nil, fmt.Errorf("mapper: same-name PO field %q conflicts with nested mapping under %s", source.Name, target.FullName())
		}
		if !typesMappable(source.Type, destination.Type, configuration.converters) {
			return nil, fmt.Errorf("mapper: no converter from %v to %v for same-name field %s", source.Type, destination.Type, target.FullName())
		}
	}
	if len(fieldNameMap) > 0 {
		copyOption.FieldNameMapping = []copier.FieldNameMapping{{
			SrcType: reflect.New(poType).Elem().Interface(),
			DstType: reflect.New(dtoType.Elem()).Elem().Interface(),
			Mapping: fieldNameMap,
		}}
	}
	for index, hook := range configuration.hooks {
		if hook.poType != poType || hook.dtoType != dtoType {
			return nil, fmt.Errorf("mapper: post hook %d has types (%v, %v), want (%v, %v)", index, hook.dtoType, hook.poType, dtoType, poType)
		}
	}
	return &ResourceMapper[DTO, PO]{
		dtoType:      dtoType,
		poType:       poType,
		copyOption:   copyOption,
		nested:       nested,
		hooks:        slices.Clone(configuration.hooks),
		resourceName: configuration.resourceName,
		identifier:   identifier,
	}, nil
}

// TryToDTO converts one PO and returns mapping errors for callers that intentionally handle them. Nil input returns (nil, nil).
func (mapper *ResourceMapper[DTO, PO]) TryToDTO(po *PO) (DTO, error) {
	var zero DTO
	if po == nil {
		return zero, nil
	}
	dtoValue := reflect.New(mapper.dtoType.Elem())
	dto := dtoValue.Interface().(DTO)
	if err := copyWithRecovery(dto, po, mapper.copyOption); err != nil {
		return zero, fmt.Errorf("mapper: copy PO to DTO: %w", err)
	}
	if len(mapper.nested) > 0 {
		if err := applyNestedProjections(dtoValue.Elem(), reflect.ValueOf(po).Elem(), mapper.nested); err != nil {
			return zero, fmt.Errorf("mapper: copy PO to DTO: %w", err)
		}
	}
	for index, hook := range mapper.hooks {
		if err := hook.invoke(po, dto); err != nil {
			return zero, fmt.Errorf("mapper: post hook %d: %w", index, err)
		}
	}
	if mapper.resourceName != nil {
		name, err := mapper.resourceName.invoke(po)
		if err != nil {
			return zero, fmt.Errorf("mapper: format resource name: %w", err)
		}
		dto.ProtoReflect().Set(mapper.identifier, protoreflect.ValueOfString(name))
	}
	clonedMessage := proto.Clone(dto)
	cloned, ok := clonedMessage.(DTO)
	if !ok {
		return zero, fmt.Errorf("mapper: cloned DTO has unexpected type %T", clonedMessage)
	}
	return cloned, nil
}

// ToDTO converts one PO and panics when TryToDTO returns an error.
func (mapper *ResourceMapper[DTO, PO]) ToDTO(po *PO) DTO {
	dto, err := mapper.TryToDTO(po)
	if err != nil {
		panic("mapper: ToDTO: " + err.Error())
	}
	return dto
}

// TryToDTOs converts a batch, preserving strict 1:1 order and returning mapping errors or nil-entry errors to the caller.
func (mapper *ResourceMapper[DTO, PO]) TryToDTOs(pos []*PO) ([]DTO, error) {
	if len(pos) == 0 {
		return nil, nil
	}
	result := make([]DTO, len(pos))
	for index, po := range pos {
		if po == nil {
			return nil, fmt.Errorf("mapper: PO at index %d is nil", index)
		}
		dto, err := mapper.TryToDTO(po)
		if err != nil {
			return nil, fmt.Errorf("mapper: PO at index %d: %w", index, err)
		}
		result[index] = dto
	}
	return result, nil
}

// ToDTOs converts a batch and panics when TryToDTOs returns an error.
func (mapper *ResourceMapper[DTO, PO]) ToDTOs(pos []*PO) []DTO {
	dtos, err := mapper.TryToDTOs(pos)
	if err != nil {
		panic("mapper: ToDTOs: " + err.Error())
	}
	return dtos
}

func resourceIdentifier(descriptor protoreflect.MessageDescriptor) (protoreflect.FieldDescriptor, error) {
	var identifier protoreflect.FieldDescriptor
	fields := descriptor.Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		behaviors, _ := proto.GetExtension(field.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
		for _, behavior := range behaviors {
			if behavior != annotations.FieldBehavior_IDENTIFIER {
				continue
			}
			if identifier != nil {
				return nil, fmt.Errorf("mapper: %s has multiple IDENTIFIER fields", descriptor.FullName())
			}
			identifier = field
		}
	}
	if identifier == nil || identifier.Kind() != protoreflect.StringKind || identifier.IsList() {
		return nil, fmt.Errorf("mapper: %s must have one singular string IDENTIFIER field", descriptor.FullName())
	}
	return identifier, nil
}

func protoGoField(dtoType reflect.Type, name protoreflect.Name) (reflect.StructField, bool) {
	for field := range dtoType.Fields() {
		for part := range strings.SplitSeq(field.Tag.Get("protobuf"), ",") {
			if strings.TrimPrefix(part, "name=") == string(name) && strings.HasPrefix(part, "name=") {
				return field, true
			}
		}
	}
	return reflect.StructField{}, false
}

func typesMappable(source, destination reflect.Type, converters []copier.TypeConverter) bool {
	if source.AssignableTo(destination) {
		return true
	}
	for _, converter := range converters {
		if reflect.TypeOf(converter.SrcType) == source && reflect.TypeOf(converter.DstType) == destination {
			return true
		}
	}
	return false
}

func expandFlattenedProjections(
	rootDescriptor protoreflect.MessageDescriptor,
	rootGoType reflect.Type,
	poType reflect.Type,
	identifier protoreflect.FieldDescriptor,
	hasResourceNameFormatter bool,
	flattens []FieldPath,
	converters []copier.TypeConverter,
	explicitSources map[string]struct{},
	seenSources map[string]struct{},
	seenTargets map[protoreflect.FullName]struct{},
	seenTargetPaths map[string]struct{},
	nestedTopLevelTargets map[protoreflect.FullName]struct{},
	fieldNameMap map[string]string,
	nested *[]nestedFieldProjection,
) error {
	topLevelByGoName := make(map[string]protoreflect.FieldDescriptor)
	topFields := rootDescriptor.Fields()
	for index := range topFields.Len() {
		topField := topFields.Get(index)
		if _, mapped := seenTargets[topField.FullName()]; mapped {
			continue
		}
		if goTopField, ok := protoGoField(rootGoType, topField.Name()); ok {
			topLevelByGoName[goTopField.Name] = topField
		}
	}

	seenFlattens := make(map[protoreflect.FullName]struct{}, len(flattens))
	flattenedSources := make(map[string]protoreflect.FullName)

	for _, subMsgPath := range flattens {
		subTarget := subMsgPath.Descriptor()
		prefixPath := subMsgPath.String()
		if subTarget == nil || strings.TrimSpace(prefixPath) == "" {
			return fmt.Errorf("mapper: flattened field %q has invalid descriptor", prefixPath)
		}
		parts := strings.Split(prefixPath, ".")
		if slices.Contains(parts, "") {
			return fmt.Errorf("mapper: flattened field path %q has an empty segment", prefixPath)
		}
		subGoElemType, err := resolveSubMessageGoType(rootDescriptor, rootGoType, prefixPath, parts, subTarget)
		if err != nil {
			return err
		}
		if _, duplicate := seenFlattens[subTarget.FullName()]; duplicate {
			return fmt.Errorf("mapper: sub-message %s is flattened more than once", subTarget.FullName())
		}
		for existingPath := range seenTargetPaths {
			if existingPath == prefixPath || strings.HasPrefix(prefixPath, existingPath+".") {
				return fmt.Errorf("mapper: flattened sub-message %q overlaps with mapped path %q", prefixPath, existingPath)
			}
		}
		seenFlattens[subTarget.FullName()] = struct{}{}

		subFields := subTarget.Message().Fields()
		for index := range subFields.Len() {
			child := subFields.Get(index)
			if _, alreadyMapped := seenTargets[child.FullName()]; alreadyMapped {
				continue
			}
			if child.Kind() == protoreflect.MessageKind && !child.IsList() && !child.IsMap() && !isWellKnownMessage(child.Message()) {
				continue
			}
			childPath := prefixPath + "." + string(child.Name())
			goChildField, ok := protoGoField(subGoElemType, child.Name())
			if !ok {
				return fmt.Errorf("mapper: generated DTO Go field for %s is missing", child.FullName())
			}
			source, ok := poType.FieldByName(goChildField.Name)
			if !ok || source.PkgPath != "" {
				continue
			}
			if _, usedExplicitly := explicitSources[source.Name]; usedExplicitly {
				continue
			}
			if prevTarget, usedByFlatten := flattenedSources[source.Name]; usedByFlatten {
				return fmt.Errorf("mapper: PO field %q matches multiple flattened fields (%s and %s)", source.Name, prevTarget, child.FullName())
			}
			if topField, existsOnTop := topLevelByGoName[source.Name]; existsOnTop {
				isFormatterOwnedIdentifier := topField.FullName() == identifier.FullName() && hasResourceNameFormatter
				if !isFormatterOwnedIdentifier {
					return fmt.Errorf("mapper: PO field %q is ambiguous between top-level %s and flattened %s", source.Name, topField.FullName(), child.FullName())
				}
			}
			childParts := append(slices.Clone(parts), string(child.Name()))
			projection, rootField, err := compileNestedProjection(
				rootDescriptor,
				rootGoType,
				source,
				childPath,
				childParts,
				child,
				converters,
			)
			if err != nil {
				return err
			}
			*nested = append(*nested, projection)
			nestedTopLevelTargets[rootField.FullName()] = struct{}{}
			fieldNameMap[source.Name] = ""
			seenSources[source.Name] = struct{}{}
			flattenedSources[source.Name] = child.FullName()
			seenTargets[child.FullName()] = struct{}{}
			seenTargetPaths[childPath] = struct{}{}
		}
	}
	return nil
}

func resolveSubMessageGoType(
	rootDescriptor protoreflect.MessageDescriptor,
	rootGoType reflect.Type,
	rawPath string,
	parts []string,
	target protoreflect.FieldDescriptor,
) (reflect.Type, error) {
	currentDesc := rootDescriptor
	currentGoType := rootGoType
	for index, part := range parts {
		fieldDesc := currentDesc.Fields().ByName(protoreflect.Name(part))
		if fieldDesc == nil {
			return nil, fmt.Errorf("mapper: flattened field %q segment %q does not exist on %s", rawPath, part, currentDesc.FullName())
		}
		if fieldDesc.Kind() != protoreflect.MessageKind || fieldDesc.IsList() || fieldDesc.IsMap() {
			return nil, fmt.Errorf("mapper: flattened field %q segment %q is not a singular message", rawPath, part)
		}
		if isWellKnownMessage(fieldDesc.Message()) {
			return nil, fmt.Errorf("mapper: flattened field %q cannot target well-known type %s", rawPath, fieldDesc.Message().FullName())
		}
		goField, ok := protoGoField(currentGoType, fieldDesc.Name())
		if !ok {
			return nil, fmt.Errorf("mapper: generated DTO Go field for %s is missing", fieldDesc.FullName())
		}
		if goField.Type.Kind() != reflect.Pointer ||
			goField.Type.Elem().Kind() != reflect.Struct ||
			!goField.Type.Implements(reflect.TypeFor[proto.Message]()) {
			return nil, fmt.Errorf("mapper: flattened Go field %s has unexpected type %v", fieldDesc.FullName(), goField.Type)
		}
		if index == len(parts)-1 && fieldDesc.FullName() != target.FullName() {
			return nil, fmt.Errorf("mapper: flattened field path %q resolved to %s, want %s", rawPath, fieldDesc.FullName(), target.FullName())
		}
		currentDesc = fieldDesc.Message()
		currentGoType = goField.Type.Elem()
	}
	return currentGoType, nil
}

func compileNestedProjection(
	rootDescriptor protoreflect.MessageDescriptor,
	rootGoType reflect.Type,
	source reflect.StructField,
	rawPath string,
	parts []string,
	target protoreflect.FieldDescriptor,
	converters []copier.TypeConverter,
) (nestedFieldProjection, protoreflect.FieldDescriptor, error) {
	currentDesc := rootDescriptor
	currentGoType := rootGoType
	steps := make([]nestedStep, 0, len(parts)-1)
	var rootField protoreflect.FieldDescriptor

	for index, part := range parts {
		fieldDesc := currentDesc.Fields().ByName(protoreflect.Name(part))
		if fieldDesc == nil {
			return nestedFieldProjection{}, nil, fmt.Errorf(
				"mapper: resource field %q segment %q does not exist on %s",
				rawPath, part, currentDesc.FullName(),
			)
		}
		if index == 0 {
			rootField = fieldDesc
		}
		goField, ok := protoGoField(currentGoType, fieldDesc.Name())
		if !ok {
			return nestedFieldProjection{}, nil, fmt.Errorf(
				"mapper: generated DTO Go field for %s is missing",
				fieldDesc.FullName(),
			)
		}
		if index < len(parts)-1 {
			if fieldDesc.Kind() != protoreflect.MessageKind || fieldDesc.IsList() || fieldDesc.IsMap() {
				return nestedFieldProjection{}, nil, fmt.Errorf(
					"mapper: intermediate field %q in %q is not a singular message",
					part, rawPath,
				)
			}
			if isWellKnownMessage(fieldDesc.Message()) {
				return nestedFieldProjection{}, nil, fmt.Errorf(
					"mapper: intermediate field %q in %q cannot traverse well-known type %s",
					part, rawPath, fieldDesc.Message().FullName(),
				)
			}
			if goField.Type.Kind() != reflect.Pointer ||
				goField.Type.Elem().Kind() != reflect.Struct ||
				!goField.Type.Implements(reflect.TypeFor[proto.Message]()) {
				return nestedFieldProjection{}, nil, fmt.Errorf(
					"mapper: intermediate Go field %s has unexpected type %v",
					fieldDesc.FullName(), goField.Type,
				)
			}
			steps = append(steps, nestedStep{
				fieldIndex: slices.Clone(goField.Index),
				elemType:   goField.Type.Elem(),
			})
			currentDesc = fieldDesc.Message()
			currentGoType = goField.Type.Elem()
			continue
		}
		if fieldDesc.FullName() != target.FullName() {
			return nestedFieldProjection{}, nil, fmt.Errorf(
				"mapper: resource field path %q resolved to %s, want %s",
				rawPath, fieldDesc.FullName(), target.FullName(),
			)
		}
		compute, ok := compileValueProjector(source.Type, goField.Type, converters)
		if !ok {
			return nestedFieldProjection{}, nil, fmt.Errorf(
				"mapper: no converter from %v to %v for %s",
				source.Type, goField.Type, target.FullName(),
			)
		}
		return nestedFieldProjection{
			sourceIndex: slices.Clone(source.Index),
			steps:       steps,
			leafIndex:   slices.Clone(goField.Index),
			compute:     compute,
		}, rootField, nil
	}
	return nestedFieldProjection{}, nil, fmt.Errorf("mapper: resource field path %q is empty", rawPath)
}

func compileValueProjector(
	sourceType, leafType reflect.Type,
	converters []copier.TypeConverter,
) (func(reflect.Value) (reflect.Value, bool, error), bool) {
	for index := len(converters) - 1; index >= 0; index-- {
		converter := converters[index]
		if reflect.TypeOf(converter.SrcType) == sourceType && reflect.TypeOf(converter.DstType) == leafType {
			invoke := converter.Fn
			return func(source reflect.Value) (reflect.Value, bool, error) {
				result, err := invoke(source.Interface())
				if err != nil {
					return reflect.Value{}, false, err
				}
				dst := reflect.ValueOf(result)
				if isDestinationAbsent(dst) {
					return reflect.Value{}, false, nil
				}
				return dst, true, nil
			}, true
		}
	}
	if !sourceType.AssignableTo(leafType) {
		return nil, false
	}
	if leafType.Implements(reflect.TypeFor[proto.Message]()) {
		return func(source reflect.Value) (reflect.Value, bool, error) {
			cloned := proto.Clone(source.Interface().(proto.Message))
			return reflect.ValueOf(cloned), true, nil
		}, true
	}
	switch leafType.Kind() {
	case reflect.Pointer:
		elemType := leafType.Elem()
		return func(source reflect.Value) (reflect.Value, bool, error) {
			copied := reflect.New(elemType)
			copied.Elem().Set(source.Elem())
			return copied, true, nil
		}, true
	case reflect.Slice:
		return func(source reflect.Value) (reflect.Value, bool, error) {
			copied := reflect.MakeSlice(leafType, source.Len(), source.Len())
			reflect.Copy(copied, source)
			return copied, true, nil
		}, true
	case reflect.Map:
		return func(source reflect.Value) (reflect.Value, bool, error) {
			copied := reflect.MakeMapWithSize(leafType, source.Len())
			iter := source.MapRange()
			for iter.Next() {
				copied.SetMapIndex(iter.Key(), iter.Value())
			}
			return copied, true, nil
		}, true
	default:
		return func(source reflect.Value) (reflect.Value, bool, error) {
			return source, true, nil
		}, true
	}
}

func applyNestedProjections(
	dtoElem, poElem reflect.Value,
	projections []nestedFieldProjection,
) error {
	for _, projection := range projections {
		source, ok := fieldByIndexIfPresent(poElem, projection.sourceIndex)
		if !ok || isSourceAbsent(source) {
			continue
		}
		destination, present, err := projection.compute(source)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		current := dtoElem
		for _, step := range projection.steps {
			subPtr := current.FieldByIndex(step.fieldIndex)
			if subPtr.IsNil() {
				subPtr.Set(reflect.New(step.elemType))
			}
			current = subPtr.Elem()
		}
		current.FieldByIndex(projection.leafIndex).Set(destination)
	}
	return nil
}

func fieldByIndexIfPresent(value reflect.Value, index []int) (reflect.Value, bool) {
	if len(index) == 1 {
		return value.Field(index[0]), true
	}
	for _, step := range index {
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return reflect.Value{}, false
			}
			value = value.Elem()
		}
		value = value.Field(step)
	}
	return value, true
}

func isSourceAbsent(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		return value.IsNil()
	case reflect.Slice, reflect.Map:
		return value.Len() == 0
	default:
		return value.IsZero()
	}
}

func isDestinationAbsent(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		return value.IsNil()
	case reflect.Slice, reflect.Map:
		return value.Len() == 0
	default:
		return false
	}
}

func isWellKnownMessage(descriptor protoreflect.MessageDescriptor) bool {
	return strings.HasPrefix(string(descriptor.FullName()), "google.protobuf.")
}

func validateAndWrapConverters(converters []copier.TypeConverter) ([]copier.TypeConverter, error) {
	wrapped := make([]copier.TypeConverter, len(converters))
	for index, converter := range converters {
		if converter.Fn == nil || converter.SrcType == nil || converter.DstType == nil {
			return nil, fmt.Errorf("mapper: converter %d is incomplete", index)
		}
		destination := reflect.TypeOf(converter.DstType)
		invoke := converter.Fn
		wrapped[index] = copier.TypeConverter{
			SrcType: converter.SrcType,
			DstType: converter.DstType,
			Fn: func(source any) (result any, err error) {
				defer func() {
					if recovered := recover(); recovered != nil {
						result = nil
						err = fmt.Errorf("converter panic: %v", recovered)
					}
				}()
				result, err = invoke(source)
				if err != nil {
					return nil, err
				}
				if result == nil || !reflect.TypeOf(result).AssignableTo(destination) {
					return nil, fmt.Errorf("converter returned %T, want %v", result, destination)
				}
				return result, nil
			},
		}
	}
	return wrapped, nil
}

func copyWithRecovery(destination, source any, option copier.Option) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("copy panic: %v", recovered)
		}
	}()
	return copier.CopyWithOption(destination, source, option)
}

func isNilFieldPath(path FieldPath) bool {
	if path == nil {
		return true
	}
	value := reflect.ValueOf(path)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

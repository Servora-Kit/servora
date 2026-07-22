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

// WithFieldMapping maps one PO Go field to one generated top-level resource field.
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

// ResourceMapper projects storage POs to generated CRUD resource messages.
type ResourceMapper[DTO proto.Message, PO any] struct {
	dtoType      reflect.Type
	poType       reflect.Type
	copyOption   copier.Option
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
	copyOption := copier.Option{
		IgnoreEmpty: false,
		DeepCopy:    true,
		Converters:  slices.Clone(configuration.converters),
	}
	fieldNameMap := make(map[string]string, len(configuration.mappings))
	seenSources := make(map[string]struct{}, len(configuration.mappings))
	seenTargets := make(map[protoreflect.FullName]struct{}, len(configuration.mappings))
	for _, mapping := range configuration.mappings {
		source, ok := poType.FieldByName(mapping.poField)
		if !ok || source.PkgPath != "" {
			return nil, fmt.Errorf("mapper: PO field %q does not exist or is unexported", mapping.poField)
		}
		target := mapping.field.Descriptor()
		if target == nil || target.ContainingMessage().FullName() != descriptor.FullName() || strings.Contains(mapping.field.String(), ".") {
			return nil, fmt.Errorf("mapper: resource field %q is not a top-level field of %s", mapping.field.String(), descriptor.FullName())
		}
		if mapping.field.String() != string(target.Name()) {
			return nil, fmt.Errorf("mapper: resource field path %q does not match descriptor %s", mapping.field.String(), target.Name())
		}
		destination, ok := protoGoField(dtoType.Elem(), target.Name())
		if !ok {
			return nil, fmt.Errorf("mapper: generated DTO Go field for %s is missing", target.FullName())
		}
		if !typesMappable(source.Type, destination.Type, configuration.converters) {
			return nil, fmt.Errorf("mapper: no converter from %v to %v for %s", source.Type, destination.Type, target.FullName())
		}
		if _, duplicate := seenSources[mapping.poField]; duplicate {
			return nil, fmt.Errorf("mapper: PO field %q is mapped more than once", mapping.poField)
		}
		if _, duplicate := seenTargets[target.FullName()]; duplicate {
			return nil, fmt.Errorf("mapper: resource field %s is mapped more than once", target.FullName())
		}
		seenSources[mapping.poField] = struct{}{}
		seenTargets[target.FullName()] = struct{}{}
		fieldNameMap[mapping.poField] = destination.Name
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
	if configuration.resourceName != nil && configuration.resourceName.poType != poType {
		return nil, fmt.Errorf("mapper: resource name formatter PO type %v, want %v", configuration.resourceName.poType, poType)
	}
	identifier, err := resourceIdentifier(descriptor)
	if err != nil {
		return nil, err
	}
	return &ResourceMapper[DTO, PO]{
		dtoType:      dtoType,
		poType:       poType,
		copyOption:   copyOption,
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
	dto := reflect.New(mapper.dtoType.Elem()).Interface().(DTO)
	if err := copyWithRecovery(dto, po, mapper.copyOption); err != nil {
		return zero, fmt.Errorf("mapper: copy PO to DTO: %w", err)
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
	for index := 0; index < dtoType.NumField(); index++ {
		field := dtoType.Field(index)
		for _, part := range strings.Split(field.Tag.Get("protobuf"), ",") {
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

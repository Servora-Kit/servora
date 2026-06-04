package mapper

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/jinzhu/copier"
)

type TypeConverter = copier.TypeConverter

// Config describes read projection mapper behavior generated from proto annotations.
type Config struct {
	FieldMapping map[string]string
	IgnoreRead   []string
}

// Apply configures a CopierMapper according to the given read projection config.
func Apply[DTO any, ENTITY any](config *Config, m *CopierMapper[DTO, ENTITY]) error {
	if m == nil {
		return fmt.Errorf("mapper: nil CopierMapper")
	}
	if config == nil {
		return nil
	}
	if len(config.FieldMapping) > 0 {
		m.WithFieldMapping(config.FieldMapping)
	}
	if len(config.IgnoreRead) > 0 {
		m.WithIgnoreRead(config.IgnoreRead...)
	}
	return nil
}

// CopierMapper is a reflection-based read projection mapper from storage entity
// type ENTITY to DTO type DTO. In Servora, DTO is usually a generated proto
// resource message.
type CopierMapper[DTO any, ENTITY any] struct {
	converters     []copier.TypeConverter
	fieldMapping   []copier.FieldNameMapping
	options        copier.Option
	postToDTOHooks []func(entity *ENTITY, dto *DTO) error
	ignoreRead     map[string]struct{}
}

func NewCopierMapper[DTO any, ENTITY any]() *CopierMapper[DTO, ENTITY] {
	return &CopierMapper[DTO, ENTITY]{
		converters: AllBuiltinConverters(),
		options: copier.Option{
			IgnoreEmpty: false,
			DeepCopy:    true,
		},
		ignoreRead: make(map[string]struct{}),
	}
}

// WithPostToDTOHook registers a function that runs after copier completes
// in TryToDTO. Use it for field-level transformations that copier cannot
// handle, such as JSON maps, edges, or nested DTOs.
func (m *CopierMapper[DTO, ENTITY]) WithPostToDTOHook(fn func(entity *ENTITY, dto *DTO) error) *CopierMapper[DTO, ENTITY] {
	m.postToDTOHooks = append(m.postToDTOHooks, fn)
	return m
}

func (m *CopierMapper[DTO, ENTITY]) AppendConverter(c copier.TypeConverter) *CopierMapper[DTO, ENTITY] {
	m.converters = append(m.converters, c)
	return m
}

func (m *CopierMapper[DTO, ENTITY]) AppendConverters(cs []copier.TypeConverter) *CopierMapper[DTO, ENTITY] {
	m.converters = append(m.converters, cs...)
	return m
}

// WithFieldMapping maps storage/entity Go field names to DTO Go field names.
func (m *CopierMapper[DTO, ENTITY]) WithFieldMapping(mapping map[string]string) *CopierMapper[DTO, ENTITY] {
	var entityZero ENTITY
	var dtoZero DTO
	for src, dst := range mapping {
		m.fieldMapping = append(m.fieldMapping,
			copier.FieldNameMapping{SrcType: entityZero, DstType: dtoZero, Mapping: map[string]string{src: dst}},
		)
	}
	return m
}

// WithIgnoreRead configures proto field names that must be cleared after read projection.
func (m *CopierMapper[DTO, ENTITY]) WithIgnoreRead(fields ...string) *CopierMapper[DTO, ENTITY] {
	for _, field := range fields {
		if field == "" {
			continue
		}
		m.ignoreRead[field] = struct{}{}
	}
	return m
}

func (m *CopierMapper[DTO, ENTITY]) buildOption() copier.Option {
	opt := m.options
	opt.Converters = m.converters
	if len(m.fieldMapping) > 0 {
		opt.FieldNameMapping = m.fieldMapping
	}
	return opt
}

// TryToDTO converts storage entity ENTITY to DTO. Returns (nil, nil) when input is nil.
func (m *CopierMapper[DTO, ENTITY]) TryToDTO(entity *ENTITY) (*DTO, error) {
	if entity == nil {
		return nil, nil
	}
	var dto DTO
	if err := copier.CopyWithOption(&dto, entity, m.buildOption()); err != nil {
		return nil, err
	}
	for _, hook := range m.postToDTOHooks {
		if err := hook(entity, &dto); err != nil {
			return nil, err
		}
	}
	if err := m.applyIgnoreRead(&dto); err != nil {
		return nil, err
	}
	return &dto, nil
}

// ToDTO converts storage entity ENTITY to DTO and panics on mapper errors.
func (m *CopierMapper[DTO, ENTITY]) ToDTO(entity *ENTITY) *DTO {
	dto, err := m.TryToDTO(entity)
	if err != nil {
		panic("mapper: ToDTO: " + err.Error())
	}
	return dto
}

func (m *CopierMapper[DTO, ENTITY]) TryToDTOList(entities []*ENTITY) ([]*DTO, error) {
	if len(entities) == 0 {
		return nil, nil
	}
	result := make([]*DTO, 0, len(entities))
	for _, entity := range entities {
		dto, err := m.TryToDTO(entity)
		if err != nil {
			return nil, err
		}
		if dto != nil {
			result = append(result, dto)
		}
	}
	return result, nil
}

func (m *CopierMapper[DTO, ENTITY]) ToDTOList(entities []*ENTITY) []*DTO {
	dtos, err := m.TryToDTOList(entities)
	if err != nil {
		panic("mapper: ToDTOList: " + err.Error())
	}
	return dtos
}

func (m *CopierMapper[DTO, ENTITY]) applyIgnoreRead(dto *DTO) error {
	if len(m.ignoreRead) == 0 || dto == nil {
		return nil
	}
	v := reflect.ValueOf(dto)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("mapper: dto must be a non-nil pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("mapper: dto must point to a struct")
	}
	fields := buildDTOFieldIndex(v.Type())
	for field := range m.ignoreRead {
		index, ok := fields[field]
		if !ok {
			continue
		}
		fv := v.Field(index)
		if fv.CanSet() {
			fv.Set(reflect.Zero(fv.Type()))
		}
	}
	return nil
}

func buildDTOFieldIndex(t reflect.Type) map[string]int {
	result := make(map[string]int, t.NumField()*3)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		result[f.Name] = i
		result[toLowerSnake(f.Name)] = i
		if name := protoFieldName(f.Tag.Get("protobuf")); name != "" {
			result[name] = i
		}
		if name := jsonFieldName(f.Tag.Get("json")); name != "" {
			result[name] = i
		}
	}
	return result
}

func protoFieldName(tag string) string {
	for _, part := range strings.Split(tag, ",") {
		if name, ok := strings.CutPrefix(part, "name="); ok {
			return name
		}
	}
	return ""
}

func jsonFieldName(tag string) string {
	if tag == "" || tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

func toLowerSnake(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	var prevLowerOrDigit bool
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if i > 0 && (prevLowerOrDigit || nextLower) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			prevLowerOrDigit = false
			continue
		}
		if unicode.IsDigit(r) {
			prevLowerOrDigit = true
		} else {
			prevLowerOrDigit = unicode.IsLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

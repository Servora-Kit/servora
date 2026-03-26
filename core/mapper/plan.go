package mapper

import (
	"fmt"
	"strings"

	"github.com/jinzhu/copier"
)

type TypeConverter = copier.TypeConverter

// ConverterKind identifies a built-in converter that the runtime can resolve.
// Values mirror the proto enum mapper.v1.ConverterKind.
type ConverterKind string

const (
	ConverterTimestampTime ConverterKind = "TIMESTAMP_TIME"
	ConverterTimePTR       ConverterKind = "TIME_PTR"
	ConverterStringPTR     ConverterKind = "STRING_PTR"
	ConverterInt64PTR      ConverterKind = "INT64_PTR"
	ConverterEnumString    ConverterKind = "ENUM_STRING"
	ConverterUUIDString    ConverterKind = "UUID_STRING"
	ConverterIntInt32      ConverterKind = "INT_INT32"
)

// MapperPlan is a declarative specification for how to configure a CopierMapper.
// Generated code produces MapperPlan values; runtime applies them.
type MapperPlan struct {
	Presets         []string
	FieldMapping    map[string]string
	FieldConverters map[string]ConverterKind
	IgnoredFields   []string
	CustomHooks     []string
}

// Validate checks that all referenced presets, field converters, and hooks
// exist without modifying any mapper.
func (p *MapperPlan) Validate(presets *PresetRegistry, hooks *HookRegistry) error {
	var errs []string

	for _, name := range p.Presets {
		if _, ok := presets.Get(name); !ok {
			errs = append(errs, fmt.Sprintf("unknown preset %q", name))
		}
	}
	for field, kind := range p.FieldConverters {
		if _, ok := builtinConverterFactories[kind]; !ok {
			errs = append(errs, fmt.Sprintf("unknown converter kind %q on field %q", kind, field))
		}
	}
	for _, name := range p.CustomHooks {
		if _, ok := hooks.Get(name); !ok {
			errs = append(errs, fmt.Sprintf("missing hook %q", name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("mapper plan validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// builtinConverterFactories maps ConverterKind to converter factory functions.
// ENUM_STRING is intentionally excluded: it requires per-enum name/value maps
// and must be registered via custom hooks or repo-level AppendConverters.
var builtinConverterFactories = map[ConverterKind]func() []TypeConverter{
	ConverterTimestampTime: NewTimestamppbConverterPair,
	ConverterTimePTR:       NewTimeConverterPair,
	ConverterStringPTR:     NewStringPointerConverterPair,
	ConverterInt64PTR:      NewInt64PointerConverterPair,
	ConverterUUIDString:    NewUUIDStringConverterPair,
	ConverterIntInt32:      NewIntInt32ConverterPair,
}

// ApplyPlan configures a CopierMapper according to the given plan.
// Go does not support type parameters on methods, so this is a package-level function.
func ApplyPlan[P any, E any](plan *MapperPlan, m *CopierMapper[P, E], presets *PresetRegistry, hooks *HookRegistry) error {
	if err := plan.Validate(presets, hooks); err != nil {
		return err
	}

	if len(plan.Presets) > 0 {
		cs, err := presets.Collect(plan.Presets...)
		if err != nil {
			return err
		}
		m.AppendConverters(cs)
	}

	for _, kind := range plan.FieldConverters {
		factory, ok := builtinConverterFactories[kind]
		if ok {
			m.AppendConverters(factory())
		}
	}

	if len(plan.FieldMapping) > 0 {
		m.WithFieldMapping(plan.FieldMapping)
	}

	for _, name := range plan.CustomHooks {
		cs := hooks.MustGet(name)
		m.AppendConverters(cs)
	}

	return nil
}

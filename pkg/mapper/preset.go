package mapper

import (
	"fmt"
	"strings"

	"github.com/jinzhu/copier"
)

// PresetRegistry manages named groups of converters.
type PresetRegistry struct {
	presets map[string]func() []copier.TypeConverter
}

func NewPresetRegistry() *PresetRegistry {
	return &PresetRegistry{presets: make(map[string]func() []copier.TypeConverter)}
}

func (r *PresetRegistry) Register(name string, factory func() []copier.TypeConverter) {
	r.presets[name] = factory
}

func (r *PresetRegistry) Get(name string) ([]copier.TypeConverter, bool) {
	f, ok := r.presets[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

// Collect gathers converters from multiple presets; returns error listing any missing.
func (r *PresetRegistry) Collect(names ...string) ([]copier.TypeConverter, error) {
	var missing []string
	var result []copier.TypeConverter
	for _, n := range names {
		cs, ok := r.Get(n)
		if !ok {
			missing = append(missing, n)
			continue
		}
		result = append(result, cs...)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("mapper: unknown presets: %s", strings.Join(missing, ", "))
	}
	return result, nil
}

// RegisterDefaults registers all built-in presets.
func (r *PresetRegistry) RegisterDefaults() {
	r.Register("proto_time", NewTimestamppbConverterPair)
	r.Register("time_ptr", NewTimeConverterPair)
	r.Register("pointer", func() []copier.TypeConverter {
		var cs []copier.TypeConverter
		cs = append(cs, NewStringPointerConverterPair()...)
		cs = append(cs, NewInt64PointerConverterPair()...)
		return cs
	})
	r.Register("common_proto_entity", func() []copier.TypeConverter {
		var cs []copier.TypeConverter
		cs = append(cs, NewTimestamppbConverterPair()...)
		cs = append(cs, NewTimeConverterPair()...)
		cs = append(cs, NewStringPointerConverterPair()...)
		cs = append(cs, NewInt64PointerConverterPair()...)
		return cs
	})
}

// DefaultPresets returns a PresetRegistry with all built-in presets registered.
func DefaultPresets() *PresetRegistry {
	r := NewPresetRegistry()
	r.RegisterDefaults()
	return r
}

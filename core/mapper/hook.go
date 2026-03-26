package mapper

import (
	"fmt"
	"strings"

	"github.com/jinzhu/copier"
)

// HookRegistry manages named custom converter hooks.
// Repos register concrete implementations; generated code references hooks by name.
type HookRegistry struct {
	hooks map[string][]copier.TypeConverter
}

func NewHookRegistry() *HookRegistry {
	return &HookRegistry{hooks: make(map[string][]copier.TypeConverter)}
}

func (r *HookRegistry) Register(name string, converters ...copier.TypeConverter) {
	r.hooks[name] = append(r.hooks[name], converters...)
}

func (r *HookRegistry) Get(name string) ([]copier.TypeConverter, bool) {
	cs, ok := r.hooks[name]
	return cs, ok
}

func (r *HookRegistry) MustGet(name string) []copier.TypeConverter {
	cs, ok := r.hooks[name]
	if !ok {
		panic(fmt.Sprintf("mapper: hook %q not registered", name))
	}
	return cs
}

// CheckRequired verifies all required hook names are registered.
func (r *HookRegistry) CheckRequired(names ...string) error {
	var missing []string
	for _, n := range names {
		if _, ok := r.hooks[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("mapper: missing hooks: %s", strings.Join(missing, ", "))
	}
	return nil
}

package bootstrap

import (
	"errors"
	"fmt"
	"reflect"

	kconfig "github.com/go-kratos/kratos/v3/config"
)

// Section is the contract implemented by configuration messages that opt into
// keyed scanning via bootstrap.Scan. Implementations are typically produced by
// protoc-gen-servora-conf from `(servora.conf.v1.section)`.
type Section interface {
	// SectionKey returns the dotted key under which the section lives in the
	// merged kratos config (e.g. "broker", "audit", "data.kafka").
	SectionKey() string
}

// OptionalSection marks a Section whose absence from the config source is
// non-fatal. When SectionOptional reports true and the key is missing, Scan
// skips both Value(key).Scan and ApplyConf for that target.
type OptionalSection interface {
	SectionOptional() bool
}

// Defaulter is the contract for messages that carry literal defaults declared
// via `(servora.conf.v1.field) = { default: ... }`.
type Defaulter interface {
	ApplyDefaults()
}

// RequiredChecker is the contract for messages that carry required-field rules.
// Typically consumed via ConfApplier; exposed for testing and direct use.
type RequiredChecker interface {
	CheckRequired() error
}

// ConfApplier is the composite contract for messages processed by
// protoc-gen-servora-conf. It runs the full post-scan sequence in a single call.
type ConfApplier interface {
	ApplyConf() error
}

// Scan loads every target from the runtime's merged kratos config. Targets that
// implement Section are scanned from Value(SectionKey()); all others are scanned
// from the whole config. ApplyConf runs only after a successful scan.
func Scan(rt *Runtime, targets ...any) error {
	if rt == nil {
		return errors.New("bootstrap: scan: nil runtime")
	}
	if rt.Config == nil {
		return errors.New("bootstrap: scan: nil config")
	}
	for i, target := range targets {
		if target == nil {
			return fmt.Errorf("bootstrap: scan target[%d]: nil", i)
		}
		if isTypedNil(target) {
			return fmt.Errorf("bootstrap: scan target[%d]: typed nil %T", i, target)
		}
		if section, ok := target.(Section); ok {
			if err := scanSectionTarget(rt.Config, i, target, section); err != nil {
				return err
			}
			continue
		}
		if err := scanConfigTarget(rt.Config, i, target); err != nil {
			return err
		}
	}
	return nil
}

func scanConfigTarget(cfg kconfig.Config, index int, target any) error {
	if err := cfg.Scan(target); err != nil {
		return fmt.Errorf("bootstrap: scan target[%d] config: %w", index, err)
	}
	if applier, ok := target.(ConfApplier); ok {
		if err := applier.ApplyConf(); err != nil {
			return fmt.Errorf("bootstrap: apply target[%d] config: %w", index, err)
		}
	}
	return nil
}

func scanSectionTarget(cfg kconfig.Config, index int, target any, section Section) error {
	key := section.SectionKey()
	if key == "" {
		return fmt.Errorf("bootstrap: scan target[%d]: empty section key", index)
	}
	if err := cfg.Value(key).Scan(target); err != nil {
		if isOptional(section) && isKeyMissing(err) {
			return nil
		}
		return fmt.Errorf("bootstrap: scan target[%d] section %q: %w", index, key, err)
	}
	if applier, ok := target.(ConfApplier); ok {
		if err := applier.ApplyConf(); err != nil {
			return fmt.Errorf("bootstrap: apply target[%d] section %q: %w", index, key, err)
		}
	}
	return nil
}

func isOptional(s Section) bool {
	if o, ok := s.(OptionalSection); ok {
		return o.SectionOptional()
	}
	return false
}

func isTypedNil(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// isKeyMissing recognises the error kratos returns when a config key is not
// present in any of the loaded sources, using the public ErrNotFound sentinel.
func isKeyMissing(err error) bool {
	return errors.Is(err, kconfig.ErrNotFound)
}

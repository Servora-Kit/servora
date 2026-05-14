package bootstrap

import (
	"errors"
	"fmt"
	"reflect"

	kconfig "github.com/go-kratos/kratos/v2/config"
)

// Section is the contract implemented by configuration messages that opt into
// keyed scanning via bootstrap.ScanSections. Implementations are typically
// produced by protoc-gen-servora-conf from `(servora.conf.v1.section)`.
type Section interface {
	// SectionKey returns the dotted key under which the section lives in the
	// merged kratos config (e.g. "broker", "audit", "data.kafka").
	SectionKey() string
}

// OptionalSection marks a Section whose absence from the config source is
// non-fatal. When SectionOptional() reports true and the key is missing,
// ScanSections skips Value(key).Scan but still invokes Defaulter so the
// receiver ends up populated with literal defaults.
type OptionalSection interface {
	SectionOptional() bool
}

// Defaulter is the contract for messages that carry literal defaults declared
// via `(servora.conf.v1.field) = { default: ... }`. ScanSections invokes it
// after a successful (or skipped-optional) Value(key).Scan.
type Defaulter interface {
	ApplyDefaults()
}

// Validator is the contract for messages that carry required-field rules. The
// returned error is propagated verbatim from ScanSections — fail-fast on the
// first failing section, no further sections are processed.
//
// The method is named ValidateConf (not Validate) to avoid colliding with the
// Validate() method that protoc-gen-validate generates on every message.
type Validator interface {
	ValidateConf() error
}

// ScanSections loads every provided section from the runtime's merged kratos
// config. For each section the sequence is:
//
//  1. If section implements OptionalSection and the key is missing, skip the
//     Value(key).Scan call. Otherwise scan; an error here is fatal.
//  2. If section implements Defaulter, call ApplyDefaults (even when scan was
//     skipped, so optional-missing still gets literal defaults).
//  3. If section implements Validator, call Validate. The first non-nil error
//     stops the iteration.
//
// Returns nil when every section completed steps 1-3 without error.
func ScanSections(rt *Runtime, sections ...Section) error {
	if rt == nil || rt.Config == nil {
		return errors.New("runtime config is nil")
	}
	for _, s := range sections {
		if isNilSection(s) {
			return errors.New("nil section in ScanSections")
		}
		key := s.SectionKey()
		if key == "" {
			return fmt.Errorf("section %T returned empty SectionKey", s)
		}
		if err := scanOneSection(rt.Config, s, key); err != nil {
			return err
		}
		if d, ok := s.(Defaulter); ok {
			d.ApplyDefaults()
		}
		if v, ok := s.(Validator); ok {
			if err := v.ValidateConf(); err != nil {
				return fmt.Errorf("section %q: %w", key, err)
			}
		}
	}
	return nil
}

// scanOneSection performs the Value(key).Scan step, honouring OptionalSection
// semantics for missing keys.
func scanOneSection(cfg kconfig.Config, s Section, key string) error {
	val := cfg.Value(key)
	// kratos config returns a non-nil Value even for missing keys; the load
	// failure surfaces via Load() on the returned errValue. We detect that
	// by attempting Scan first, then deciding based on optional-ness.
	err := val.Scan(s)
	if err == nil {
		return nil
	}
	if isOptional(s) && isKeyMissing(err) {
		return nil
	}
	return fmt.Errorf("scan section %q: %w", key, err)
}

func isOptional(s Section) bool {
	if o, ok := s.(OptionalSection); ok {
		return o.SectionOptional()
	}
	return false
}

// isNilSection covers both the bare nil interface and a typed-nil pointer
// (the classic Go interface-nil pitfall).
func isNilSection(s Section) bool {
	if s == nil {
		return true
	}
	v := reflect.ValueOf(s)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// isKeyMissing recognises the error kratos returns when a config key is not
// present in any of the loaded sources, using the public ErrNotFound sentinel.
func isKeyMissing(err error) bool {
	return errors.Is(err, kconfig.ErrNotFound)
}

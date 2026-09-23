package bootstrap

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	kconfig "github.com/go-kratos/kratos/v3/config"
	"google.golang.org/protobuf/proto"
)

// ConfApplier 是配置消息解码后执行默认、必填及值约束的唯一入口。
type ConfApplier interface {
	Apply() error
}

var (
	acronymBoundary = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	wordBoundary    = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// Scan 按描述符标记读取配置段，其余 target 读取整份配置。
// 缺失配置段跳过解码与 Apply；存在段的解码或 Apply 错误均返回。
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
		if message, ok := target.(proto.Message); ok && isSection(message) {
			if err := scanSectionTarget(rt.Config, i, target, sectionKey(message)); err != nil {
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
		if err := applier.Apply(); err != nil {
			return fmt.Errorf("bootstrap: apply target[%d] config: %w", index, err)
		}
	}
	return nil
}

func scanSectionTarget(cfg kconfig.Config, index int, target any, key string) error {
	if err := cfg.Value(key).Scan(target); err != nil {
		if errors.Is(err, kconfig.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("bootstrap: scan target[%d] section %q: %w", index, key, err)
	}
	if applier, ok := target.(ConfApplier); ok {
		if err := applier.Apply(); err != nil {
			return fmt.Errorf("bootstrap: apply target[%d] section %q: %w", index, key, err)
		}
	}
	return nil
}

func isSection(message proto.Message) bool {
	return proto.GetExtension(message.ProtoReflect().Descriptor().Options(), confv1.E_Section).(bool)
}

func sectionKey(message proto.Message) string {
	name := string(message.ProtoReflect().Descriptor().Name())
	name = acronymBoundary.ReplaceAllString(name, "${1}_${2}")
	return strings.ToLower(wordBoundary.ReplaceAllString(name, "${1}_${2}"))
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

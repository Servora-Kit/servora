// Package protoreach 提供 Proto 消息的生成期可达性检查。
// 旧的 NeedsCascade 仅遍历普通单值消息，保留供原调用方使用；
// 配置生成器通过 ConfigChild 和 NeedsConfig 遍历集合元素与 map 值。
package protoreach

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// NeedsCascade 保留原有的单值消息递归规则，不改变其他插件的行为。
// memo 同时缓存结果并避免自引用循环；oneof 的处理由调用方决定。
func NeedsCascade(
	md protoreflect.MessageDescriptor,
	pred func(protoreflect.FieldDescriptor) bool,
	memo map[protoreflect.FullName]bool,
) bool {
	if md == nil {
		return false
	}
	if v, ok := memo[md.FullName()]; ok {
		return v
	}
	memo[md.FullName()] = false

	result := false
	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if pred(fd) {
			result = true
			break
		}
		if fd.Kind() != protoreflect.MessageKind || fd.IsList() || fd.IsMap() {
			continue
		}
		sub := fd.Message()
		if IsWellKnown(sub) {
			continue
		}
		if NeedsCascade(sub, pred, memo) {
			result = true
			break
		}
	}
	memo[md.FullName()] = result
	return result
}

// ConfigChild 返回字段实际包含的自定义子消息，支持普通字段、集合和 map 值。
// 内建消息与 map-entry 合成类型不继续展开。
func ConfigChild(fd protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if fd == nil {
		return nil
	}
	if fd.IsMap() {
		fd = fd.MapValue()
	}
	if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
		return nil
	}
	md := fd.Message()
	if md == nil || md.IsMapEntry() || IsWellKnown(md) {
		return nil
	}
	return md
}

// NeedsConfig 检查所有字段形态中的配置子消息，不改变旧的 NeedsCascade 规则。
func NeedsConfig(md protoreflect.MessageDescriptor, pred func(protoreflect.FieldDescriptor) bool, memo map[protoreflect.FullName]bool) bool {
	return needsConfig(md, pred, memo, make(map[protoreflect.FullName]bool))
}

func needsConfig(md protoreflect.MessageDescriptor, pred func(protoreflect.FieldDescriptor) bool, memo, active map[protoreflect.FullName]bool) bool {
	if md == nil || md.IsMapEntry() || IsWellKnown(md) {
		return false
	}
	if active[md.FullName()] {
		return false
	}
	if value, ok := memo[md.FullName()]; ok {
		return value
	}
	active[md.FullName()] = true
	result := false
	for i := range md.Fields().Len() {
		fd := md.Fields().Get(i)
		if pred(fd) || needsConfig(ConfigChild(fd), pred, memo, active) {
			result = true
			break
		}
	}
	delete(active, md.FullName())
	memo[md.FullName()] = result
	return result
}

// IsWellKnown 判断消息是否属于 google.protobuf 命名空间。
func IsWellKnown(md protoreflect.MessageDescriptor) bool {
	return strings.HasPrefix(string(md.FullName()), "google.protobuf.")
}

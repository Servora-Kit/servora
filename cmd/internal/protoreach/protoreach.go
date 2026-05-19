// Package protoreach provides a predicate-parameterized tree-walk over proto
// message descriptors. It answers one question: "does any field in this
// message's subtree satisfy a caller-supplied predicate?" The walk handles
// cycle-guarding (self-referential messages), diamond-caching (shared
// sub-messages evaluated once), and skips well-known google.protobuf.* types,
// lists, and maps.
//
// Designed for protoc-gen-servora-* plugins that need transitive reachability
// decisions at codegen time (e.g. "does this message need a cascading
// ApplyDefaults?" or "does it need a cascading CheckRequired?").
package protoreach

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// NeedsCascade reports whether md must carry a generated cascade method
// based on pred. Returns true when any field in md satisfies pred, or
// when any singular non-well-known message-typed field transitively does.
//
// memo doubles as the recursion guard and the result cache: an entry is
// written as false on entry (breaking self-referential cycles at the
// back-edge) and overwritten with the real result on exit, so shared
// (diamond) subtrees are evaluated correctly. Callers pass a fresh map.
//
// Oneof members ARE traversed for reachability. However, emitters must
// decide independently whether to allocate them (ApplyDefaults must not;
// CheckRequired only checks if set).
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
	for i := 0; i < fields.Len(); i++ {
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

// IsWellKnown reports whether md is in the google.protobuf.* namespace.
func IsWellKnown(md protoreflect.MessageDescriptor) bool {
	return strings.HasPrefix(string(md.FullName()), "google.protobuf.")
}

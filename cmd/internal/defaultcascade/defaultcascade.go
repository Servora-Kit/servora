// Package defaultcascade provides the shared codegen-time decision used by
// protoc-gen-servora-conf to emit cascading ApplyDefaults(). It is a
// sibling of cmd/internal/optionmerge in organization only — it does NOT
// reuse optionmerge's whole-message replace semantics.
package defaultcascade

import (
	"strings"

	confv1 "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// NeedsApplyDefaults reports whether md must carry a generated
// ApplyDefaults(): true if md has any field with a non-empty
// (servora.conf.v1.field) default, OR any singular message-typed field
// (excluding well-known types, lists, maps) whose message transitively
// needs one.
//
// memo doubles as the recursion guard and the result cache: an entry is
// written as false on entry (breaking self-referential cycles at the
// back-edge) and overwritten with the real result on exit, so shared
// (diamond) subtrees are evaluated correctly. Callers pass a fresh map.
//
// Note: oneof members are intentionally NOT skipped here. NeedsApplyDefaults
// returns true for a container that transitively reaches a default via an
// oneof member. However, the emitter (cascadeChildren in
// protoc-gen-servora-conf) MUST NOT allocate oneof members, since doing so
// would violate the at-most-one-case invariant.
//
// Consequence: if a future oneof member's message itself carries defaults,
// those defaults will NOT be applied automatically — the outer container's
// ApplyDefaults is emitted, but the inner oneof allocation is skipped. This
// is a known limitation; handle such a case explicitly at its call site.
func NeedsApplyDefaults(md protoreflect.MessageDescriptor, memo map[protoreflect.FullName]bool) bool {
	if md == nil {
		return false
	}
	if v, ok := memo[md.FullName()]; ok {
		return v
	}
	memo[md.FullName()] = false // in-progress: cycle back-edge sees false

	result := false
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fieldHasDefault(fd) {
			result = true
			break
		}
		if fd.Kind() != protoreflect.MessageKind || fd.IsList() || fd.IsMap() {
			continue
		}
		sub := fd.Message()
		if isWellKnown(sub) {
			continue
		}
		if NeedsApplyDefaults(sub, memo) {
			result = true
			break
		}
	}
	memo[md.FullName()] = result
	return result
}

func fieldHasDefault(fd protoreflect.FieldDescriptor) bool {
	opts := fd.Options()
	if opts == nil || !proto.HasExtension(opts, confv1.E_Field) {
		return false
	}
	r, _ := proto.GetExtension(opts, confv1.E_Field).(*confv1.FieldRule)
	return r != nil && r.GetDefault() != ""
}

// isWellKnown reports whether md is in the google.protobuf.* namespace
// only. Other google.* families (api, rpc) are intentionally not covered:
// servora's Bootstrap tree does not reference them, and widening this
// would be speculative.
func isWellKnown(md protoreflect.MessageDescriptor) bool {
	return strings.HasPrefix(string(md.FullName()), "google.protobuf.")
}

// Package optionmerge provides shared merge logic for the three protoc-gen-servora-*
// plugins (authn, authz, audit). All three follow identical semantics:
//
//   - method-level rule with mode != 0 (UNSPECIFIED) fully replaces the service default,
//   - method-level rule absent or mode == 0 inherits the service default,
//   - if neither contributes a non-zero mode, no rule applies.
//
// "Mode" is conventionally the first field (field number 1) in each rule proto message.
package optionmerge

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// modeFieldNumber is the proto field number used for the mode enum across all
// three rule types (AuthnRule.mode, AuthzRule.mode, AuditRule.mode).
const modeFieldNumber protoreflect.FieldNumber = 1

// Merge returns the effective rule after applying service-level / method-level
// merge semantics. The result is always a deep clone so callers may mutate it.
//
// hasMethod should be true if the method-level annotation was present in the
// proto descriptor (even if methodRule itself is non-nil, hasMethod == false
// means the annotation was absent and the caller pre-allocated a nil value).
//
// Returns (result, true) when a usable rule exists; (zero, false) when neither
// side declares a non-UNSPECIFIED mode.
func Merge[T proto.Message](svcDefault, methodRule T, hasMethod bool) (T, bool) {
	var zero T

	if hasMethod && !isNil(methodRule) && modeNonZero(methodRule) {
		return proto.Clone(methodRule).(T), true
	}
	if !isNil(svcDefault) && modeNonZero(svcDefault) {
		return proto.Clone(svcDefault).(T), true
	}
	return zero, false
}

// modeNonZero returns true if field number 1 (the mode enum) has a non-zero value.
func modeNonZero(m proto.Message) bool {
	md := m.ProtoReflect().Descriptor()
	fd := md.Fields().ByNumber(modeFieldNumber)
	if fd == nil {
		return false
	}
	val := m.ProtoReflect().Get(fd)
	return val.Enum() != 0
}

// isNil returns true if the interface holds a nil pointer.
func isNil[T proto.Message](m T) bool {
	return proto.Message(m) == nil || !m.ProtoReflect().IsValid()
}

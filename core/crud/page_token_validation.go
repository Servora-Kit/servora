package crud

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
)

const contextFingerprintDomain = "servora.crud.query-context.v1"

// ContextFingerprintInput contains the complete canonical List context.
// IncludeTotal is intentionally absent because Count does not affect membership or ordering.
type ContextFingerprintInput struct {
	ResourceType     string
	Collection       string
	Filter           FilterExpression
	Order            FinalOrder
	ScopeFingerprint []byte
}

// ComputeContextFingerprint hashes length-delimited canonical query components.
func ComputeContextFingerprint(input ContextFingerprintInput) [sha256.Size]byte {
	hasher := sha256.New()
	writeFingerprintPart(hasher, []byte(contextFingerprintDomain))
	writeFingerprintPart(hasher, []byte(input.ResourceType))
	writeFingerprintPart(hasher, []byte(input.Collection))
	writeFingerprintPart(hasher, []byte(input.Filter.String()))
	terms := input.Order.terms
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(terms)))
	writeFingerprintPart(hasher, count[:])
	for _, term := range terms {
		writeFingerprintPart(hasher, []byte(term.binding.key))
		writeFingerprintPart(hasher, []byte(term.binding.fieldPath))
		writeFingerprintPart(hasher, []byte(term.binding.profileID))
		writeFingerprintPart(hasher, []byte(term.binding.logicalType))
		writeFingerprintPart(hasher, []byte{byte(term.direction)})
		if term.binding.nullable {
			writeFingerprintPart(hasher, []byte{1})
		} else {
			writeFingerprintPart(hasher, []byte{0})
		}
	}
	writeFingerprintPart(hasher, input.ScopeFingerprint)
	var result [sha256.Size]byte
	copy(result[:], hasher.Sum(nil))
	return result
}

type fingerprintWriter interface {
	Write([]byte) (int, error)
}

func writeFingerprintPart(writer fingerprintWriter, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

// CursorValue is one validated, backend-neutral cursor value.
type CursorValue struct {
	logicalType            LogicalType
	null                   bool
	stringValue            string
	boolValue              bool
	intValue               int64
	uintValue              uint64
	doubleValue            float64
	bytesValue             []byte
	timestamp              time.Time
	timestampOffsetSeconds int32
	duration               durationScalar
}

// LogicalType returns the expected canonical type.
func (value CursorValue) LogicalType() LogicalType { return value.logicalType }

// IsNull reports whether this value represents a backend NULL.
func (value CursorValue) IsNull() bool { return value.null }

// StringValue returns a string cursor.
func (value CursorValue) StringValue() (string, bool) {
	return value.stringValue, !value.null && value.logicalType == LogicalString
}

// BoolValue returns a bool cursor.
func (value CursorValue) BoolValue() (bool, bool) {
	return value.boolValue, !value.null && value.logicalType == LogicalBool
}

// Int64Value returns a signed integer or enum cursor.
func (value CursorValue) Int64Value() (int64, bool) {
	valid := value.logicalType == LogicalInt32 || value.logicalType == LogicalInt64 || value.logicalType == LogicalEnum
	return value.intValue, !value.null && valid
}

// Uint64Value returns an unsigned integer cursor.
func (value CursorValue) Uint64Value() (uint64, bool) {
	valid := value.logicalType == LogicalUint32 || value.logicalType == LogicalUint64
	return value.uintValue, !value.null && valid
}

// DoubleValue returns a finite floating-point cursor.
func (value CursorValue) DoubleValue() (float64, bool) {
	valid := value.logicalType == LogicalFloat32 || value.logicalType == LogicalFloat64
	return value.doubleValue, !value.null && valid
}

// BytesValue returns a copy of a bytes cursor.
func (value CursorValue) BytesValue() ([]byte, bool) {
	if value.null || value.logicalType != LogicalBytes {
		return nil, false
	}
	return append([]byte(nil), value.bytesValue...), true
}

// TimestampValue returns a timestamp cursor with its original storage offset.
func (value CursorValue) TimestampValue() (time.Time, bool) {
	if value.null || value.logicalType != LogicalTimestamp {
		return time.Time{}, false
	}
	return value.timestamp.In(time.FixedZone("", int(value.timestampOffsetSeconds))), true
}

// DurationValue returns a copy of a duration cursor.
func (value CursorValue) DurationValue() (*durationpb.Duration, bool) {
	if value.null || value.logicalType != LogicalDuration {
		return nil, false
	}
	return value.duration.proto(), true
}

// ValidatePageTokenPayload validates context and positional cursor types before database access.
func ValidatePageTokenPayload(
	payload *crudpb.PageTokenPayload,
	expectedFingerprint [sha256.Size]byte,
	order FinalOrder,
) ([]CursorValue, error) {
	if payload == nil {
		return nil, invalidPageToken("page_token", "payload is nil")
	}
	if payload.GetVersion() != CurrentPageTokenVersion {
		return nil, invalidPageToken("page_token", "unsupported payload version %d", payload.GetVersion())
	}
	fingerprint := payload.GetContextFingerprint()
	if len(fingerprint) != sha256.Size {
		return nil, invalidPageToken("page_token", "context fingerprint must be %d bytes", sha256.Size)
	}
	if subtle.ConstantTimeCompare(fingerprint, expectedFingerprint[:]) != 1 {
		return nil, invalidPageToken("page_token", "context fingerprint does not match the current query")
	}
	if len(payload.GetCursor()) != len(order.terms) {
		return nil, invalidPageToken(
			"page_token",
			"cursor count %d does not match final order count %d",
			len(payload.GetCursor()),
			len(order.terms),
		)
	}
	values := make([]CursorValue, len(order.terms))
	for index, term := range order.terms {
		value, err := validateCursorValue(payload.GetCursor()[index], term.binding)
		if err != nil {
			return nil, invalidPageToken("page_token", "cursor[%d]: %v", index, err)
		}
		values[index] = value
	}
	return values, nil
}

func validateCursorValue(cursor *crudpb.CursorValue, binding OrderBinding) (CursorValue, error) {
	if cursor == nil || cursor.GetValue() == nil {
		return CursorValue{}, fmt.Errorf("oneof value is unset")
	}
	logicalType := binding.logicalType
	if !logicalType.valid() {
		return CursorValue{}, fmt.Errorf("binding logical type %q is invalid", logicalType)
	}
	if logicalType != LogicalTimestamp && cursor.GetTimestampOffsetSeconds() != 0 {
		return CursorValue{}, fmt.Errorf("timestamp offset is set for logical type %q", logicalType)
	}
	if nullValue, ok := cursor.GetValue().(*crudpb.CursorValue_NullValue); ok {
		if nullValue == nil {
			return CursorValue{}, fmt.Errorf("oneof value is unset")
		}
		if nullValue.NullValue != structpb.NullValue_NULL_VALUE {
			return CursorValue{}, fmt.Errorf("invalid NullValue %d", nullValue.NullValue)
		}
		if cursor.GetTimestampOffsetSeconds() != 0 {
			return CursorValue{}, fmt.Errorf("NULL cursor has a timestamp offset")
		}
		if !binding.nullable {
			return CursorValue{}, fmt.Errorf("binding %q is non-null", binding.key)
		}
		return CursorValue{logicalType: logicalType, null: true}, nil
	}

	value := CursorValue{logicalType: logicalType}
	switch logicalType {
	case LogicalString:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_StringValue)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		value.stringValue = arm.StringValue
	case LogicalBool:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_BoolValue)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		value.boolValue = arm.BoolValue
	case LogicalEnum:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_Int64Value)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		if arm.Int64Value < math.MinInt32 || arm.Int64Value > math.MaxInt32 {
			return CursorValue{}, fmt.Errorf("enum number %d is out of int32 range", arm.Int64Value)
		}
		value.intValue = arm.Int64Value
	case LogicalInt64:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_Int64Value)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		value.intValue = arm.Int64Value
	case LogicalInt32:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_Int64Value)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		if arm.Int64Value < math.MinInt32 || arm.Int64Value > math.MaxInt32 {
			return CursorValue{}, fmt.Errorf("int32 value %d is out of range", arm.Int64Value)
		}
		value.intValue = arm.Int64Value
	case LogicalUint64:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_Uint64Value)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		value.uintValue = arm.Uint64Value
	case LogicalUint32:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_Uint64Value)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		if arm.Uint64Value > math.MaxUint32 {
			return CursorValue{}, fmt.Errorf("uint32 value %d is out of range", arm.Uint64Value)
		}
		value.uintValue = arm.Uint64Value
	case LogicalFloat32, LogicalFloat64:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_DoubleValue)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		if math.IsNaN(arm.DoubleValue) || math.IsInf(arm.DoubleValue, 0) {
			return CursorValue{}, fmt.Errorf("floating-point value must be finite")
		}
		if logicalType == LogicalFloat32 {
			converted := float32(arm.DoubleValue)
			if float64(converted) != arm.DoubleValue || (arm.DoubleValue == 0 && math.Signbit(float64(converted)) != math.Signbit(arm.DoubleValue)) {
				return CursorValue{}, fmt.Errorf("float32 value %g is not exactly representable", arm.DoubleValue)
			}
		}
		value.doubleValue = arm.DoubleValue
	case LogicalBytes:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_BytesValue)
		if !ok || arm == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		value.bytesValue = append([]byte(nil), arm.BytesValue...)
	case LogicalTimestamp:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_TimestampValue)
		if !ok || arm == nil || arm.TimestampValue == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		if err := arm.TimestampValue.CheckValid(); err != nil {
			return CursorValue{}, fmt.Errorf("invalid timestamp: %w", err)
		}
		value.timestamp = arm.TimestampValue.AsTime()
		offset := cursor.GetTimestampOffsetSeconds()
		if offset <= -24*60*60 || offset >= 24*60*60 || offset%60 != 0 {
			return CursorValue{}, fmt.Errorf("timestamp offset %d is outside the supported whole-minute range", offset)
		}
		value.timestampOffsetSeconds = offset

	case LogicalDuration:
		arm, ok := cursor.GetValue().(*crudpb.CursorValue_DurationValue)
		if !ok || arm == nil || arm.DurationValue == nil {
			return CursorValue{}, cursorArmMismatch(logicalType)
		}
		if err := arm.DurationValue.CheckValid(); err != nil {
			return CursorValue{}, fmt.Errorf("invalid duration: %w", err)
		}
		value.duration = durationScalarFromProto(arm.DurationValue)
	default:
		return CursorValue{}, fmt.Errorf("logical type %q is unsupported", logicalType)
	}
	return value, nil
}

func cursorArmMismatch(logicalType LogicalType) error {
	return fmt.Errorf("oneof arm does not match logical type %q", logicalType)
}

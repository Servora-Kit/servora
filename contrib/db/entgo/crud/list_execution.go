package crud

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	corecrud "github.com/Servora-Kit/servora/core/crud"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const cursorAliasPrefix = "__servora_cursor_"

// EntListQuery is the generated Ent query surface required by List.
// Generated schemas must enable Ent's sql/modifier feature.
type EntListQuery[PO any, Self any, Selection any] interface {
	Clone() Self
	Modify(...func(*sql.Selector)) Selection
	Limit(int) Self
	Offset(int) Self
	All(context.Context) ([]PO, error)
	Count(context.Context) (int, error)
}

// ResolvedListQuery contains one request's validated backend binding and token context.
// It is produced and consumed inside List; no public Resolve-to-List workflow is exposed.
type ResolvedListQuery struct {
	filter      SelectorPredicate
	order       corecrud.FinalOrder
	fingerprint [32]byte
	cursor      []corecrud.CursorValue
}

// List resolves one backend-neutral query and executes it on the supplied Ent builder.
func List[PO any, Selection any, Q EntListQuery[PO, Q, Selection]](
	ctx context.Context,
	builder Q,
	query corecrud.ListQuery,
	fields *ListFields[PO],
	scopeFingerprint []byte,
) (corecrud.ListResult[PO], error) {
	if ctx == nil {
		return corecrud.ListResult[PO]{}, internalAdapterError("list", "context is nil")
	}
	if isNilGeneric(builder) {
		return corecrud.ListResult[PO]{}, internalAdapterError("list", "Ent query builder is nil")
	}
	if fields == nil {
		return corecrud.ListResult[PO]{}, internalAdapterError("list", "ListFields is nil")
	}

	resolved, err := fields.resolveList(query, scopeFingerprint)
	if err != nil {
		return corecrud.ListResult[PO]{}, err
	}
	countBuilder := builder.Clone()
	pageBuilder := builder.Clone()
	if isNilGeneric(countBuilder) || isNilGeneric(pageBuilder) {
		return corecrud.ListResult[PO]{}, internalAdapterError("list", "Ent query Clone returned nil")
	}

	applyFilter := func(selector *sql.Selector) {
		if resolved.filter != nil {
			selector.Where(resolved.filter(selector))
		}
	}
	var totalSize *int64
	if query.IncludeTotal() {
		countBuilder.Modify(applyFilter)
		count, countErr := countBuilder.Count(ctx)
		if countErr != nil {
			return corecrud.ListResult[PO]{}, countErr
		}
		value := int64(count)
		totalSize = &value
	}

	if query.Skip() > int64(maxInt()) {
		return corecrud.ListResult[PO]{}, invalidFieldValueError("skip exceeds platform int range")
	}
	pageBuilder.Modify(func(selector *sql.Selector) {
		applyFilter(selector)
		if len(resolved.cursor) != 0 {
			selector.Where(fields.keysetPredicate(selector, resolved.order, resolved.cursor))
		}
		fields.applyOrder(selector, resolved.order)
		fields.appendCursorSelects(selector, resolved.order)
	})
	pageBuilder.Offset(int(query.Skip()))
	pageBuilder.Limit(int(query.PageSize()) + 1)
	items, err := pageBuilder.All(ctx)
	if err != nil {
		return corecrud.ListResult[PO]{}, err
	}

	pageSize := int(query.PageSize())
	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}
	var nextPageToken string
	if hasNext {
		cursor, cursorErr := fields.extractCursor(items[len(items)-1], resolved.order)
		if cursorErr != nil {
			return corecrud.ListResult[PO]{}, cursorErr
		}
		nextPageToken, cursorErr = query.EncodePageToken(resolved.fingerprint, cursor)
		if cursorErr != nil {
			return corecrud.ListResult[PO]{}, cursorErr
		}
	}
	return corecrud.NewListResult(query, items, nextPageToken, totalSize)
}

func (fields *ListFields[PO]) resolveList(
	query corecrud.ListQuery,
	scopeFingerprint []byte,
) (ResolvedListQuery, error) {
	filter, err := fields.resolveFilter(query.Filter())
	if err != nil {
		return ResolvedListQuery{}, err
	}
	order, err := fields.orderAssembler.Resolve(query.OrderBy())
	if err != nil {
		return ResolvedListQuery{}, err
	}
	fingerprint := corecrud.ComputeContextFingerprint(corecrud.ContextFingerprintInput{
		ResourceType: query.ResourceType(), Collection: query.Collection(), Filter: query.Filter(),
		Order: order, ScopeFingerprint: append([]byte(nil), scopeFingerprint...),
	})
	var cursor []corecrud.CursorValue
	if payload := query.PageTokenPayload(); payload != nil {
		cursor, err = corecrud.ValidatePageTokenPayload(payload, fingerprint, order)
		if err != nil {
			return ResolvedListQuery{}, err
		}
	}
	return ResolvedListQuery{filter: filter, order: order, fingerprint: fingerprint, cursor: cursor}, nil
}

func (fields *ListFields[PO]) applyOrder(selector *sql.Selector, order corecrud.FinalOrder) {
	for _, term := range order.Terms() {
		source := fields.orderSources[term.Binding().Key()]
		if source.nullable {
			selector.OrderExprFunc(func(builder *sql.Builder) {
				builder.WriteString("CASE WHEN ")
				fields.writeOrderExpression(builder, selector, source)
				builder.WriteString(" IS NULL THEN 1 ELSE 0 END")
			})
		}
		fields.orderByValue(selector, term, source)
	}
}

func (fields *ListFields[PO]) orderByValue(
	selector *sql.Selector,
	term corecrud.FinalOrderTerm,
	source fieldBinding,
) {
	direction := term.Direction()
	if source.customOrder != nil || source.logicalType == corecrud.LogicalString {
		selector.OrderExprFunc(func(builder *sql.Builder) {
			if source.logicalType == corecrud.LogicalString {
				fields.writeBinaryOrderExpression(builder, selector, source)
			} else {
				fields.writeOrderExpression(builder, selector, source)
			}
			if direction == corecrud.OrderDescending {
				builder.WriteString(" DESC")
			}
		})
		return
	}
	column := selector.C(source.column)
	if direction == corecrud.OrderDescending {
		selector.OrderBy(sql.Desc(column))
	} else {
		selector.OrderBy(column)
	}
}

func (fields *ListFields[PO]) appendCursorSelects(selector *sql.Selector, order corecrud.FinalOrder) {
	for index, term := range order.Terms() {
		source := fields.orderSources[term.Binding().Key()]
		if source.customOrder != nil {
			selector.AppendSelectExprAs(source.customOrder(selector), cursorAlias(index))
		} else {
			selector.AppendSelectAs(selector.C(source.column), cursorAlias(index))
		}
	}
}

func (fields *ListFields[PO]) keysetPredicate(
	selector *sql.Selector,
	order corecrud.FinalOrder,
	cursor []corecrud.CursorValue,
) *sql.Predicate {
	terms := order.Terms()
	alternatives := make([]*sql.Predicate, 0, len(terms))
	for index, term := range terms {
		conjunction := make([]*sql.Predicate, 0, index+1)
		for prefix := 0; prefix < index; prefix++ {
			conjunction = append(conjunction, fields.cursorComparison(
				selector, terms[prefix], cursor[prefix], corecrud.FilterOperatorEqual,
			))
		}
		operator := corecrud.FilterOperatorGreater
		if term.Direction() == corecrud.OrderDescending {
			operator = corecrud.FilterOperatorLess
		}
		conjunction = append(conjunction, fields.cursorComparison(selector, term, cursor[index], operator))
		alternatives = append(alternatives, sql.And(conjunction...))
	}
	return sql.Or(alternatives...)
}

func (fields *ListFields[PO]) cursorComparison(
	selector *sql.Selector,
	term corecrud.FinalOrderTerm,
	cursor corecrud.CursorValue,
	operator corecrud.FilterOperator,
) *sql.Predicate {
	source := fields.orderSources[term.Binding().Key()]
	if !source.nullable {
		return fields.orderValueComparison(selector, source, operator, cursorArgument(cursor))
	}
	if operator == corecrud.FilterOperatorEqual {
		if cursor.IsNull() {
			return fields.orderValueNullPredicate(selector, source, true)
		}
		return sql.And(
			fields.orderValueNullPredicate(selector, source, false),
			fields.orderValueComparison(selector, source, corecrud.FilterOperatorEqual, cursorArgument(cursor)),
		)
	}
	if cursor.IsNull() {
		return sql.False()
	}
	return sql.Or(
		fields.orderValueNullPredicate(selector, source, true),
		sql.And(
			fields.orderValueNullPredicate(selector, source, false),
			fields.orderValueComparison(selector, source, operator, cursorArgument(cursor)),
		),
	)
}

func (fields *ListFields[PO]) orderValueComparison(
	selector *sql.Selector,
	source fieldBinding,
	operator corecrud.FilterOperator,
	argument any,
) *sql.Predicate {
	if source.customOrder == nil {
		predicate, _ := columnComparison(
			selector.C(source.column), operator, argument,
			source.logicalType == corecrud.LogicalString,
		)
		return predicate
	}
	op, _ := comparisonOp(operator)
	return sql.P(func(builder *sql.Builder) {
		if source.logicalType == corecrud.LogicalString {
			fields.writeBinaryOrderExpression(builder, selector, source)
		} else {
			fields.writeOrderExpression(builder, selector, source)
		}
		builder.WriteOp(op).Arg(argument)
	})
}

func (fields *ListFields[PO]) orderValueNullPredicate(
	selector *sql.Selector,
	source fieldBinding,
	isNull bool,
) *sql.Predicate {
	if source.customOrder == nil {
		if isNull {
			return sql.IsNull(selector.C(source.column))
		}
		return sql.NotNull(selector.C(source.column))
	}
	return sql.P(func(builder *sql.Builder) {
		fields.writeOrderExpression(builder, selector, source)
		if isNull {
			builder.WriteString(" IS NULL")
		} else {
			builder.WriteString(" IS NOT NULL")
		}
	})
}

func (fields *ListFields[PO]) writeOrderExpression(
	builder *sql.Builder,
	selector *sql.Selector,
	source fieldBinding,
) {
	if source.customOrder != nil {
		builder.Join(source.customOrder(selector))
		return
	}
	builder.Ident(selector.C(source.column))
}

func (fields *ListFields[PO]) writeBinaryOrderExpression(
	builder *sql.Builder,
	selector *sql.Selector,
	source fieldBinding,
) {
	if source.customOrder == nil {
		writeBinaryStringExpression(builder, selector.C(source.column))
		return
	}
	switch builder.Dialect() {
	case dialect.MySQL:
		builder.WriteString("BINARY (").Join(source.customOrder(selector)).WriteString(")")
	case dialect.Postgres:
		builder.WriteString("(").Join(source.customOrder(selector)).WriteString(`) COLLATE "C"`)
	default:
		builder.WriteString("(").Join(source.customOrder(selector)).WriteString(") COLLATE BINARY")
	}
}

func (fields *ListFields[PO]) extractCursor(item PO, order corecrud.FinalOrder) ([]*crudpb.CursorValue, error) {
	terms := order.Terms()
	cursor := make([]*crudpb.CursorValue, len(terms))
	var reader interface {
		Value(string) (ent.Value, error)
	}
	for index, term := range terms {
		source := fields.orderSources[term.Binding().Key()]
		var (
			raw any
			err error
		)
		if source.cursorExtractor != nil {
			raw, err = source.cursorExtractor(any(item), cursorAlias(index))
		} else {
			if reader == nil {
				var ok bool
				reader, ok = any(item).(interface {
					Value(string) (ent.Value, error)
				})
				if !ok || isNilGeneric(item) {
					return nil, internalAdapterError("page_token", "result type %T does not expose Ent Value(name)", item)
				}
			}
			raw, err = reader.Value(cursorAlias(index))
		}
		if err != nil {
			return nil, internalAdapterError("page_token", "read cursor[%d]: %v", index, err)
		}
		cursor[index], err = encodeCursorValue(raw, source, term.Binding().Nullable())
		if err != nil {
			return nil, internalAdapterError("page_token", "encode cursor[%d]: %v", index, err)
		}
	}
	return cursor, nil
}

func encodeCursorValue(raw any, source fieldBinding, nullable bool) (*crudpb.CursorValue, error) {
	if raw == nil {
		if !nullable {
			return nil, fmt.Errorf("non-null cursor key %q returned NULL", source.column)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_NullValue{NullValue: structpb.NullValue_NULL_VALUE}}, nil
	}
	if source.cursorConverter != nil {
		converted, err := source.cursorConverter(raw)
		if err != nil {
			return nil, fmt.Errorf("converter: %w", err)
		}
		raw = converted
	}
	switch source.logicalType {
	case corecrud.LogicalString:
		value, err := asString(raw)
		if err != nil {
			return nil, err
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_StringValue{StringValue: value}}, nil
	case corecrud.LogicalBool:
		value, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("value %T is not bool", raw)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_BoolValue{BoolValue: value}}, nil
	case corecrud.LogicalEnum, corecrud.LogicalInt32, corecrud.LogicalInt64:
		value, err := asInt64(raw)
		if err != nil {
			return nil, err
		}
		if (source.logicalType == corecrud.LogicalEnum || source.logicalType == corecrud.LogicalInt32) &&
			(value < math.MinInt32 || value > math.MaxInt32) {
			return nil, fmt.Errorf("value %d is outside int32 range", value)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_Int64Value{Int64Value: value}}, nil
	case corecrud.LogicalUint32, corecrud.LogicalUint64:
		value, err := asUint64(raw)
		if err != nil {
			return nil, err
		}
		if source.logicalType == corecrud.LogicalUint32 && value > math.MaxUint32 {
			return nil, fmt.Errorf("value %d is outside uint32 range", value)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_Uint64Value{Uint64Value: value}}, nil
	case corecrud.LogicalFloat32, corecrud.LogicalFloat64:
		value, err := asFloat64(raw)
		if err != nil {
			return nil, err
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("floating-point value must be finite")
		}
		if source.logicalType == corecrud.LogicalFloat32 && float64(float32(value)) != value {
			return nil, fmt.Errorf("value %g is not exactly representable as float32", value)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_DoubleValue{DoubleValue: value}}, nil
	case corecrud.LogicalBytes:
		value, ok := raw.([]byte)
		if !ok {
			return nil, fmt.Errorf("value %T is not []byte", raw)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_BytesValue{BytesValue: append([]byte(nil), value...)}}, nil
	case corecrud.LogicalTimestamp:
		value, ok := raw.(time.Time)
		if !ok {
			return nil, fmt.Errorf("value %T is not time.Time", raw)
		}
		timestamp := timestamppb.New(value)
		if err := timestamp.CheckValid(); err != nil {
			return nil, fmt.Errorf("timestamp: %w", err)
		}
		_, offset := value.Zone()
		if offset <= -24*60*60 || offset >= 24*60*60 || offset%60 != 0 {
			return nil, fmt.Errorf("timestamp timezone offset %d is outside the supported whole-minute range", offset)
		}
		return &crudpb.CursorValue{
			Value:                  &crudpb.CursorValue_TimestampValue{TimestampValue: timestamp},
			TimestampOffsetSeconds: int32(offset),
		}, nil

	case corecrud.LogicalDuration:
		value, ok := raw.(time.Duration)
		if !ok {
			return nil, fmt.Errorf("value %T is not time.Duration", raw)
		}
		duration := durationpb.New(value)
		if err := duration.CheckValid(); err != nil {
			return nil, fmt.Errorf("duration: %w", err)
		}
		return &crudpb.CursorValue{Value: &crudpb.CursorValue_DurationValue{DurationValue: duration}}, nil
	default:
		return nil, fmt.Errorf("logical type %q is unsupported", source.logicalType)
	}
}

func cursorArgument(cursor corecrud.CursorValue) any {
	if cursor.IsNull() {
		return nil
	}
	if value, ok := cursor.StringValue(); ok {
		return value
	}
	if value, ok := cursor.BoolValue(); ok {
		return value
	}
	if value, ok := cursor.Int64Value(); ok {
		return value
	}
	if value, ok := cursor.Uint64Value(); ok {
		return value
	}
	if value, ok := cursor.DoubleValue(); ok {
		return value
	}
	if value, ok := cursor.BytesValue(); ok {
		return value
	}
	if value, ok := cursor.TimestampValue(); ok {
		return value
	}
	if value, ok := cursor.DurationValue(); ok {
		return value.AsDuration()
	}
	return nil
}

func cursorAlias(index int) string { return fmt.Sprintf("%s%d", cursorAliasPrefix, index) }

func asString(value any) (string, error) {
	switch value := value.(type) {
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	default:
		return "", fmt.Errorf("value %T is not string-compatible", value)
	}
}

func asInt64(value any) (int64, error) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return 0, fmt.Errorf("value is nil")
	}
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		unsigned := reflected.Uint()
		if unsigned > math.MaxInt64 {
			return 0, fmt.Errorf("value %d exceeds int64", unsigned)
		}
		return int64(unsigned), nil
	default:
		return 0, fmt.Errorf("value %T is not an integer", value)
	}
}

func asUint64(value any) (uint64, error) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return 0, fmt.Errorf("value is nil")
	}
	switch reflected.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflected.Uint(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		signed := reflected.Int()
		if signed < 0 {
			return 0, fmt.Errorf("value %d is negative", signed)
		}
		return uint64(signed), nil
	default:
		return 0, fmt.Errorf("value %T is not an unsigned integer", value)
	}
}

func asFloat64(value any) (float64, error) {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return 0, fmt.Errorf("value is nil")
	}
	switch reflected.Kind() {
	case reflect.Float32, reflect.Float64:
		return reflected.Float(), nil
	default:
		return 0, fmt.Errorf("value %T is not floating-point", value)
	}
}

func isNilGeneric[T any](value T) bool {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func maxInt() int { return int(^uint(0) >> 1) }

func invalidFieldValueError(format string, args ...any) error {
	return crudpb.ErrorCrudErrorReasonInvalidFieldValue("%s", "skip: "+fmt.Sprintf(format, args...))
}

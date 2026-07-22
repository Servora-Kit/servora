package crud

import (
	"errors"
	"fmt"
	"go.einride.tech/aip/filtering"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	largeIntFunction  = "__servora_int64"
	largeUintFunction = "__servora_uint64"
)

// FilterNodeKind identifies one immutable filter AST node.
type FilterNodeKind uint8

const (
	FilterNodeInvalid FilterNodeKind = iota
	FilterNodeComparison
	FilterNodeAnd
	FilterNodeOr
	FilterNodeHas
)

// FilterOperator is a supported AIP filter operator.
type FilterOperator string

const (
	FilterOperatorEqual        FilterOperator = "="
	FilterOperatorNotEqual     FilterOperator = "!="
	FilterOperatorLess         FilterOperator = "<"
	FilterOperatorLessEqual    FilterOperator = "<="
	FilterOperatorGreater      FilterOperator = ">"
	FilterOperatorGreaterEqual FilterOperator = ">="
	FilterOperatorHas          FilterOperator = ":"
)

// FilterValueKind identifies a typed filter literal.
type FilterValueKind uint8

const (
	FilterValueInvalid FilterValueKind = iota
	FilterValueNull
	FilterValueString
	FilterValueBool
	FilterValueEnum
	FilterValueInt64
	FilterValueUint64
	FilterValueDouble
	FilterValueTimestamp
	FilterValueDuration
)

// FilterExpression is an immutable, typed, normalized AIP filter.
type FilterExpression struct {
	root       FilterNode
	normalized string
	nodeCount  int
	depth      int
	orTerms    int
}
type FilterNode struct {
	kind     FilterNodeKind
	operator FilterOperator
	field    FieldPlan
	value    FilterValue
	children []FilterNode
}

type durationScalar struct {
	seconds int64
	nanos   int32
}

func durationScalarFromProto(value *durationpb.Duration) durationScalar {
	return durationScalar{seconds: value.GetSeconds(), nanos: value.GetNanos()}
}

func (value durationScalar) proto() *durationpb.Duration {
	return &durationpb.Duration{Seconds: value.seconds, Nanos: value.nanos}
}

// FilterValue is one immutable, descriptor-checked filter literal.
type FilterValue struct {
	kind       FilterValueKind
	text       string
	boolValue  bool
	intValue   int64
	uintValue  uint64
	floatValue float64
	enumValue  protoreflect.EnumNumber
	timestamp  time.Time
	duration   durationScalar
}

// Empty reports whether no filter was provided.
func (f FilterExpression) Empty() bool { return f.root.kind == FilterNodeInvalid }

// Root returns the root filter node.
func (f FilterExpression) Root() FilterNode { return f.root }

// String returns the canonical normalized filter text.
func (f FilterExpression) String() string { return f.normalized }

// NodeCount returns the number of AST nodes.
func (f FilterExpression) NodeCount() int { return f.nodeCount }

// Depth returns the maximum AST depth.
func (f FilterExpression) Depth() int { return f.depth }

// Kind returns the node kind.
func (n FilterNode) Kind() FilterNodeKind { return n.kind }

// ORTerms returns the total number of alternatives in OR expressions.
func (f FilterExpression) ORTerms() int { return f.orTerms }

// Operator returns the comparison or has operator.
func (n FilterNode) Operator() FilterOperator { return n.operator }

// Field returns the referenced resource field and whether this node has one.
func (n FilterNode) Field() (FieldPlan, bool) {
	return n.field, n.kind == FilterNodeComparison || n.kind == FilterNodeHas
}

// Value returns the node's typed literal.
func (n FilterNode) Value() FilterValue { return n.value }

// Children returns a copy of logical child nodes.
func (n FilterNode) Children() []FilterNode { return slices.Clone(n.children) }

// Kind returns the literal kind.
func (v FilterValue) Kind() FilterValueKind { return v.kind }

// StringValue returns a string or enum symbol.
func (v FilterValue) StringValue() (string, bool) {
	return v.text, v.kind == FilterValueString || v.kind == FilterValueEnum
}

// BoolValue returns a bool literal.
func (v FilterValue) BoolValue() (bool, bool) { return v.boolValue, v.kind == FilterValueBool }

// Int64Value returns a signed integer literal.
func (v FilterValue) Int64Value() (int64, bool) { return v.intValue, v.kind == FilterValueInt64 }

// Uint64Value returns an unsigned integer literal.
func (v FilterValue) Uint64Value() (uint64, bool) { return v.uintValue, v.kind == FilterValueUint64 }

// DoubleValue returns a finite floating-point literal.
func (v FilterValue) DoubleValue() (float64, bool) { return v.floatValue, v.kind == FilterValueDouble }

// EnumNumber returns the descriptor-resolved enum number.
func (v FilterValue) EnumNumber() (protoreflect.EnumNumber, bool) {
	return v.enumValue, v.kind == FilterValueEnum
}

// TimestampValue returns a timestamp literal.
func (v FilterValue) TimestampValue() (time.Time, bool) {
	return v.timestamp, v.kind == FilterValueTimestamp
}

// DurationValue returns a copy of a protobuf duration literal.
func (v FilterValue) DurationValue() (*durationpb.Duration, bool) {
	if v.kind != FilterValueDuration {
		return nil, false
	}
	return v.duration.proto(), true
}

type filterResource interface {
	ResourceType() string
	Descriptor() protoreflect.MessageDescriptor
	Field(string) (FieldPlan, bool)
	QueryablePaths() []string
}

type integerLiteralReplacement struct {
	start int
	end   int
	value string
}

func normalizeLargeIntegerLiterals(value string) (string, error) {
	var lexer filtering.Lexer
	lexer.Init(value)
	tokens := make([]filtering.Token, 0, 16)
	for {
		token, err := lexer.Lex()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("lex: %w", err)
		}
		tokens = append(tokens, token)
		if token.Type == filtering.TokenTypeText && (token.Value == largeIntFunction || token.Value == largeUintFunction) {
			return "", fmt.Errorf("function %q is not supported", token.Value)
		}
	}
	replacements := make([]integerLiteralReplacement, 0, 1)
	for index, token := range tokens {
		if token.Type != filtering.TokenTypeNumber && token.Type != filtering.TokenTypeHexNumber {
			continue
		}
		if nextSignificantToken(tokens, index).Type == filtering.TokenTypeDot {
			continue
		}
		magnitude, err := strconv.ParseUint(token.Value, 0, 64)
		if err == nil && magnitude <= math.MaxInt64 {
			continue
		}
		if err != nil {
			if numberError, ok := err.(*strconv.NumError); !ok || numberError.Err != strconv.ErrRange {
				continue
			}
		}
		start := int(token.Position.Offset)
		literal := token.Value
		function := largeUintFunction
		if index > 0 && tokens[index-1].Type == filtering.TokenTypeMinus &&
			int(tokens[index-1].Position.Offset)+len(tokens[index-1].Value) == start {
			start = int(tokens[index-1].Position.Offset)
			literal = "-" + literal
			function = largeIntFunction
		}
		replacements = append(replacements, integerLiteralReplacement{
			start: start,
			end:   int(token.Position.Offset) + len(token.Value),
			value: function + "(" + strconv.Quote(literal) + ")",
		})
	}
	if len(replacements) == 0 {
		return value, nil
	}
	var result strings.Builder
	result.Grow(len(value) + len(replacements)*16)
	position := 0
	for _, replacement := range replacements {
		result.WriteString(value[position:replacement.start])
		result.WriteString(replacement.value)
		position = replacement.end
	}
	result.WriteString(value[position:])
	return result.String(), nil
}

func nextSignificantToken(tokens []filtering.Token, index int) filtering.Token {
	for index++; index < len(tokens); index++ {
		if tokens[index].Type != filtering.TokenTypeWhitespace {
			return tokens[index]
		}
	}
	return filtering.Token{}
}
func parseFilter(value string, resource filterResource, syntacticDepth int) (FilterExpression, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return FilterExpression{}, nil
	}
	parseValue, err := normalizeLargeIntegerLiterals(value)
	if err != nil {
		return FilterExpression{}, err
	}

	var parser filtering.Parser
	parser.Init(parseValue)
	parsed, err := parser.Parse()
	if err != nil {
		return FilterExpression{}, fmt.Errorf("parse: %w", err)
	}
	if err := checkFilterExpression(parsed, resource); err != nil {
		return FilterExpression{}, err
	}
	root, err := convertFilterNode(parsed.GetExpr(), resource)
	if err != nil {
		return FilterExpression{}, err
	}
	nodeCount, logicalDepth := filterStats(root)
	return FilterExpression{
		root:       root,
		normalized: renderFilterNode(root, FilterNodeInvalid),
		nodeCount:  nodeCount,
		depth:      max(logicalDepth, syntacticDepth),
		orTerms:    filterORTerms(root),
	}, nil
}

func checkFilterExpression(parsed *exprpb.ParsedExpr, resource filterResource) error {
	options := []filtering.DeclarationOption{filtering.DeclareStandardFunctions()}
	options = append(options, filtering.DeclareProtoMessageIdents(
		dynamicpb.NewMessage(resource.Descriptor()),
		filtering.WithFilterableFields(resource.QueryablePaths()...),
	)...)
	declarations, err := filtering.NewDeclarations(options...)
	if err != nil {
		return fmt.Errorf("declare fields: %w", err)
	}
	if requiresManualFilterCheck(parsed.GetExpr()) {
		return nil
	}
	var checker filtering.Checker
	checker.Init(parsed.GetExpr(), parsed.GetSourceInfo(), declarations)
	if _, err := checker.Check(); err != nil {
		return fmt.Errorf("type check: %w", err)
	}
	return nil
}

func requiresManualFilterCheck(expression *exprpb.Expr) bool {
	if identifier := expression.GetIdentExpr(); identifier != nil {
		switch identifier.GetName() {
		case "null", "true", "false":
			return true
		}
	}
	if selection := expression.GetSelectExpr(); selection != nil && requiresManualFilterCheck(selection.GetOperand()) {
		return true
	}
	call := expression.GetCallExpr()
	if call.GetFunction() == filtering.FunctionHas || call.GetFunction() == largeIntFunction || call.GetFunction() == largeUintFunction {
		return true
	}
	for _, argument := range call.GetArgs() {
		if requiresManualFilterCheck(argument) {
			return true
		}
	}
	return false
}

func convertFilterNode(expression *exprpb.Expr, resource filterResource) (FilterNode, error) {
	call := expression.GetCallExpr()
	if call == nil {
		return FilterNode{}, fmt.Errorf("bare literals and field references are not supported")
	}

	switch call.GetFunction() {
	case filtering.FunctionAnd, filtering.FunctionOr:
		if len(call.GetArgs()) < 2 {
			return FilterNode{}, fmt.Errorf("%s requires at least two operands", call.GetFunction())
		}
		children := make([]FilterNode, len(call.GetArgs()))
		for index, argument := range call.GetArgs() {
			child, err := convertFilterNode(argument, resource)
			if err != nil {
				return FilterNode{}, err
			}
			children[index] = child
		}
		kind := FilterNodeAnd
		if call.GetFunction() == filtering.FunctionOr {
			kind = FilterNodeOr
		}
		return FilterNode{kind: kind, children: children}, nil
	case filtering.FunctionEquals, filtering.FunctionNotEquals,
		filtering.FunctionLessThan, filtering.FunctionLessEquals,
		filtering.FunctionGreaterThan, filtering.FunctionGreaterEquals:
		return convertComparison(call, resource)
	case filtering.FunctionHas:
		return convertHas(call, resource)
	case filtering.FunctionNot:
		return FilterNode{}, fmt.Errorf("NOT is not supported")
	default:
		return FilterNode{}, fmt.Errorf("function %q is not supported", call.GetFunction())
	}
}

func convertComparison(call *exprpb.Expr_Call, resource filterResource) (FilterNode, error) {
	if len(call.GetArgs()) != 2 {
		return FilterNode{}, fmt.Errorf("operator %s requires two operands", call.GetFunction())
	}
	field, err := resolveFilterField(call.GetArgs()[0], resource)
	if err != nil {
		return FilterNode{}, err
	}
	if field.descriptor.IsList() || field.descriptor.IsMap() {
		return FilterNode{}, fmt.Errorf("field %q requires the contains operator", field.path)
	}
	operator := FilterOperator(call.GetFunction())
	literal, err := parseFilterValue(call.GetArgs()[1], field.descriptor)
	if err != nil {
		return FilterNode{}, fmt.Errorf("field %q: %w", field.path, err)
	}
	if err := validateFilterOperator(operator, field.descriptor, literal); err != nil {
		return FilterNode{}, fmt.Errorf("field %q: %w", field.path, err)
	}
	return FilterNode{kind: FilterNodeComparison, operator: operator, field: field, value: literal}, nil
}

func convertHas(call *exprpb.Expr_Call, resource filterResource) (FilterNode, error) {
	if len(call.GetArgs()) != 2 {
		return FilterNode{}, fmt.Errorf("operator : requires two operands")
	}
	field, err := resolveFilterField(call.GetArgs()[0], resource)
	if err != nil {
		return FilterNode{}, err
	}
	if !field.descriptor.IsList() || field.descriptor.IsMap() || !isFilterScalar(field.descriptor.Kind()) {
		return FilterNode{}, fmt.Errorf("field %q does not support repeated-value containment", field.path)
	}
	literalExpression := call.GetArgs()[1]
	if constant := literalExpression.GetConstExpr(); constant != nil && field.descriptor.Kind() != protoreflect.StringKind {
		if _, ok := constant.GetConstantKind().(*exprpb.Constant_StringValue); ok {
			literalExpression = &exprpb.Expr{
				ExprKind: &exprpb.Expr_IdentExpr{
					IdentExpr: &exprpb.Expr_Ident{Name: constant.GetStringValue()},
				},
			}
		}
	}
	literal, err := parseFilterValue(literalExpression, field.descriptor)
	if err != nil {
		return FilterNode{}, fmt.Errorf("field %q: %w", field.path, err)
	}
	if literal.kind == FilterValueNull {
		return FilterNode{}, fmt.Errorf("contains does not accept null")
	}
	return FilterNode{kind: FilterNodeHas, operator: FilterOperatorHas, field: field, value: literal}, nil
}

func resolveFilterField(expression *exprpb.Expr, resource filterResource) (FieldPlan, error) {
	path, ok := filterFieldPath(expression)
	if !ok {
		return FieldPlan{}, fmt.Errorf("left operand must be a resource field path")
	}
	field, ok := resource.Field(path)
	if !ok || field.HasBehavior(annotations.FieldBehavior_IDENTIFIER) || field.HasBehavior(annotations.FieldBehavior_INPUT_ONLY) {
		return FieldPlan{}, fmt.Errorf("field %q is not queryable", path)
	}
	return field, nil
}

func filterFieldPath(expression *exprpb.Expr) (string, bool) {
	if identifier := expression.GetIdentExpr(); identifier != nil {
		return identifier.GetName(), identifier.GetName() != ""
	}
	selection := expression.GetSelectExpr()
	if selection == nil || selection.GetTestOnly() {
		return "", false
	}
	parent, ok := filterFieldPath(selection.GetOperand())
	if !ok {
		return "", false
	}
	return parent + "." + selection.GetField(), true
}

func parseFilterValue(expression *exprpb.Expr, field protoreflect.FieldDescriptor) (FilterValue, error) {
	if identifier := expression.GetIdentExpr(); identifier != nil {
		name := identifier.GetName()
		switch {
		case name == "null":
			return FilterValue{kind: FilterValueNull}, nil
		case field.Kind() == protoreflect.BoolKind && (name == "true" || name == "false"):
			return FilterValue{kind: FilterValueBool, boolValue: name == "true"}, nil
		case field.Kind() == protoreflect.EnumKind:
			value := field.Enum().Values().ByName(protoreflect.Name(name))
			if value == nil {
				return FilterValue{}, fmt.Errorf("unknown enum value %q", name)
			}
			return FilterValue{kind: FilterValueEnum, text: name, enumValue: value.Number()}, nil
		default:
			return FilterValue{}, fmt.Errorf("identifier %q is not a valid literal", name)
		}
	}

	if constant := expression.GetConstExpr(); constant != nil {
		return parseConstantFilterValue(constant, field)
	}
	if call := expression.GetCallExpr(); call != nil {
		return parseWellKnownFilterValue(call, field)
	}
	return FilterValue{}, fmt.Errorf("unsupported literal")
}

func parseConstantFilterValue(constant *exprpb.Constant, field protoreflect.FieldDescriptor) (FilterValue, error) {
	switch field.Kind() {
	case protoreflect.StringKind:
		if _, ok := constant.GetConstantKind().(*exprpb.Constant_StringValue); !ok {
			return FilterValue{}, fmt.Errorf("expected string literal")
		}
		return FilterValue{kind: FilterValueString, text: constant.GetStringValue()}, nil
	case protoreflect.BoolKind:
		if _, ok := constant.GetConstantKind().(*exprpb.Constant_BoolValue); !ok {
			return FilterValue{}, fmt.Errorf("expected bool literal")
		}
		return FilterValue{kind: FilterValueBool, boolValue: constant.GetBoolValue()}, nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		value, ok := integerConstant(constant)
		if !ok {
			return FilterValue{}, fmt.Errorf("expected integer literal")
		}
		if isInt32Kind(field.Kind()) && (value < math.MinInt32 || value > math.MaxInt32) {
			return FilterValue{}, fmt.Errorf("integer %d is outside int32 range", value)
		}
		return FilterValue{kind: FilterValueInt64, intValue: value}, nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		value, ok := integerConstant(constant)
		if !ok || value < 0 {
			return FilterValue{}, fmt.Errorf("expected non-negative integer literal")
		}
		unsigned := uint64(value)
		if isUint32Kind(field.Kind()) && unsigned > math.MaxUint32 {
			return FilterValue{}, fmt.Errorf("integer %d is outside uint32 range", unsigned)
		}
		return FilterValue{kind: FilterValueUint64, uintValue: unsigned}, nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		value, ok := numericConstant(constant)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return FilterValue{}, fmt.Errorf("expected finite numeric literal")
		}
		if field.Kind() == protoreflect.FloatKind && math.Abs(value) > math.MaxFloat32 {
			return FilterValue{}, fmt.Errorf("number %g is outside float range", value)
		}
		return FilterValue{kind: FilterValueDouble, floatValue: value}, nil
	case protoreflect.EnumKind:
		return FilterValue{}, fmt.Errorf("expected enum symbol")
	case protoreflect.MessageKind:
		if isTimestampField(field) || isDurationField(field) {
			if _, ok := constant.GetConstantKind().(*exprpb.Constant_StringValue); !ok {
				return FilterValue{}, fmt.Errorf("expected string-form well-known literal")
			}
			return parseWellKnownString(constant.GetStringValue(), field)
		}
		fallthrough
	default:
		return FilterValue{}, fmt.Errorf("field type %s is not filterable", field.Kind())
	}
}

func parseWellKnownFilterValue(call *exprpb.Expr_Call, field protoreflect.FieldDescriptor) (FilterValue, error) {
	if len(call.GetArgs()) != 1 || call.GetArgs()[0].GetConstExpr() == nil {
		return FilterValue{}, fmt.Errorf("literal function requires one string argument")
	}
	constant := call.GetArgs()[0].GetConstExpr()
	if _, ok := constant.GetConstantKind().(*exprpb.Constant_StringValue); !ok {
		return FilterValue{}, fmt.Errorf("literal function requires one string argument")
	}
	value := constant.GetStringValue()
	switch call.GetFunction() {
	case filtering.FunctionTimestamp:
		if !isTimestampField(field) {
			return FilterValue{}, fmt.Errorf("timestamp literal does not match %s", field.Kind())
		}
		return parseWellKnownString(value, field)
	case filtering.FunctionDuration:
		if !isDurationField(field) {
			return FilterValue{}, fmt.Errorf("duration literal does not match %s", field.Kind())
		}
		return parseWellKnownString(value, field)
	case largeIntFunction:
		return parseLargeIntegerValue(value, field, true)
	case largeUintFunction:
		return parseLargeIntegerValue(value, field, false)
	default:
		return FilterValue{}, fmt.Errorf("function %q is not a literal", call.GetFunction())
	}
}

func parseLargeIntegerValue(value string, field protoreflect.FieldDescriptor, signed bool) (FilterValue, error) {
	if isSignedIntegerKind(field.Kind()) && signed {
		parsed, err := strconv.ParseInt(value, 0, fieldIntegerBits(field.Kind()))
		if err != nil {
			return FilterValue{}, fmt.Errorf("signed integer out of range: %w", err)
		}
		return FilterValue{kind: FilterValueInt64, intValue: parsed}, nil
	}
	if isUnsignedIntegerKind(field.Kind()) && !signed {
		parsed, err := strconv.ParseUint(value, 0, fieldIntegerBits(field.Kind()))
		if err != nil {
			return FilterValue{}, fmt.Errorf("unsigned integer out of range: %w", err)
		}
		return FilterValue{kind: FilterValueUint64, uintValue: parsed}, nil
	}
	if field.Kind() == protoreflect.FloatKind || field.Kind() == protoreflect.DoubleKind {
		bits := 64
		if field.Kind() == protoreflect.FloatKind {
			bits = 32
		}
		parsed, err := strconv.ParseFloat(value, bits)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return FilterValue{}, fmt.Errorf("finite floating-point literal required")
		}
		return FilterValue{kind: FilterValueDouble, floatValue: parsed}, nil
	}
	return FilterValue{}, fmt.Errorf("integer literal does not match %s", field.Kind())
}

func parseWellKnownString(value string, field protoreflect.FieldDescriptor) (FilterValue, error) {
	if isTimestampField(field) {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return FilterValue{}, fmt.Errorf("invalid RFC3339 timestamp: %w", err)
		}
		return FilterValue{kind: FilterValueTimestamp, timestamp: parsed.UTC()}, nil
	}
	if isDurationField(field) {
		parsed, err := parseProtoDuration(value)
		if err != nil {
			return FilterValue{}, err
		}
		return FilterValue{kind: FilterValueDuration, duration: durationScalarFromProto(parsed)}, nil
	}
	return FilterValue{}, fmt.Errorf("field is not a timestamp or duration")
}

func parseProtoDuration(value string) (*durationpb.Duration, error) {
	if !strings.HasSuffix(value, "s") {
		return nil, fmt.Errorf("duration must use protobuf seconds syntax")
	}
	text := strings.TrimSuffix(value, "s")
	negative := strings.HasPrefix(text, "-")
	if negative {
		text = strings.TrimPrefix(text, "-")
	}
	if text == "" || strings.HasPrefix(text, "+") {
		return nil, fmt.Errorf("invalid protobuf duration %q", value)
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || parts[0] == "" || !decimalDigits(parts[0]) {
		return nil, fmt.Errorf("invalid protobuf duration %q", value)
	}
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid protobuf duration %q", value)
	}
	var nanos int32
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 0 || len(fraction) > 9 || !decimalDigits(fraction) {
			return nil, fmt.Errorf("invalid protobuf duration %q", value)
		}
		parsed, err := strconv.ParseInt(fraction, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid protobuf duration %q", value)
		}
		nanos = int32(parsed)
		for range 9 - len(fraction) {
			nanos *= 10
		}
	}
	if negative {
		seconds = -seconds
		nanos = -nanos
	}
	result := &durationpb.Duration{Seconds: seconds, Nanos: nanos}
	if err := result.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid protobuf duration: %w", err)
	}
	return result, nil
}

func decimalDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateFilterOperator(operator FilterOperator, field protoreflect.FieldDescriptor, value FilterValue) error {
	if value.kind == FilterValueNull {
		if operator != FilterOperatorEqual && operator != FilterOperatorNotEqual {
			return fmt.Errorf("null only supports = and !=")
		}
		return nil
	}
	if field.Kind() == protoreflect.BoolKind || field.Kind() == protoreflect.EnumKind {
		if operator != FilterOperatorEqual && operator != FilterOperatorNotEqual {
			return fmt.Errorf("%s only supports = and !=", field.Kind())
		}
	}
	return nil
}

func integerConstant(constant *exprpb.Constant) (int64, bool) {
	_, ok := constant.GetConstantKind().(*exprpb.Constant_Int64Value)
	return constant.GetInt64Value(), ok
}

func numericConstant(constant *exprpb.Constant) (float64, bool) {
	switch constant.GetConstantKind().(type) {
	case *exprpb.Constant_Int64Value:
		return float64(constant.GetInt64Value()), true
	case *exprpb.Constant_DoubleValue:
		return constant.GetDoubleValue(), true
	default:
		return 0, false
	}
}

func isInt32Kind(kind protoreflect.Kind) bool {
	return kind == protoreflect.Int32Kind || kind == protoreflect.Sint32Kind || kind == protoreflect.Sfixed32Kind
}

func isUint32Kind(kind protoreflect.Kind) bool {
	return kind == protoreflect.Uint32Kind || kind == protoreflect.Fixed32Kind
}

func isSignedIntegerKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return true
	default:
		return false
	}
}

func isUnsignedIntegerKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	default:
		return false
	}
}

func fieldIntegerBits(kind protoreflect.Kind) int {
	if isInt32Kind(kind) || isUint32Kind(kind) {
		return 32
	}
	return 64
}

func isFilterScalar(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.BoolKind, protoreflect.EnumKind, protoreflect.Int32Kind,
		protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.Int64Kind,
		protoreflect.Sint64Kind, protoreflect.Sfixed64Kind, protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind, protoreflect.StringKind:
		return true
	default:
		return false
	}
}

func isTimestampField(field protoreflect.FieldDescriptor) bool {
	return field.Kind() == protoreflect.MessageKind && field.Message().FullName() == "google.protobuf.Timestamp"
}

func isDurationField(field protoreflect.FieldDescriptor) bool {
	return field.Kind() == protoreflect.MessageKind && field.Message().FullName() == "google.protobuf.Duration"
}

func filterStats(node FilterNode) (count, depth int) {
	count, depth = 1, 1
	for _, child := range node.children {
		childCount, childDepth := filterStats(child)
		count += childCount
		depth = max(depth, childDepth+1)
	}
	return count, depth
}

func filterParenthesisDepth(value string) (int, error) {
	var lexer filtering.Lexer
	lexer.Init(value)
	current, maximum := 0, 0
	for {
		token, err := lexer.Lex()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("lex: %w", err)
		}
		switch token.Type {
		case filtering.TokenTypeLeftParen:
			current++
			maximum = max(maximum, current)
		case filtering.TokenTypeRightParen:
			current--
			if current < 0 {
				return 0, fmt.Errorf("unexpected closing parenthesis")
			}
		}
	}
	if current != 0 {
		return 0, fmt.Errorf("unclosed parenthesis")
	}
	return maximum, nil
}

func filterORTerms(node FilterNode) int {
	if node.kind == FilterNodeOr {
		return countORLeaves(node)
	}
	total := 0
	for _, child := range node.children {
		total += filterORTerms(child)
	}
	return total
}

func countORLeaves(node FilterNode) int {
	if node.kind != FilterNodeOr {
		return 1
	}
	total := 0
	for _, child := range node.children {
		total += countORLeaves(child)
	}
	return total
}

func renderFilterNode(node FilterNode, parent FilterNodeKind) string {
	var result string
	switch node.kind {
	case FilterNodeAnd, FilterNodeOr:
		separator := " AND "
		if node.kind == FilterNodeOr {
			separator = " OR "
		}
		parts := make([]string, len(node.children))
		for index, child := range node.children {
			parts[index] = renderFilterNode(child, node.kind)
		}
		result = strings.Join(parts, separator)
	case FilterNodeComparison, FilterNodeHas:
		result = node.field.path + " " + string(node.operator) + " " + renderFilterValue(node.value)
	}
	if parent == FilterNodeAnd && node.kind == FilterNodeOr {
		return "(" + result + ")"
	}
	return result
}

func renderFilterValue(value FilterValue) string {
	switch value.kind {
	case FilterValueNull:
		return "null"
	case FilterValueString:
		return strconv.Quote(value.text)
	case FilterValueBool:
		return strconv.FormatBool(value.boolValue)
	case FilterValueEnum:
		return value.text
	case FilterValueInt64:
		return strconv.FormatInt(value.intValue, 10)
	case FilterValueUint64:
		return strconv.FormatUint(value.uintValue, 10)
	case FilterValueDouble:
		return strconv.FormatFloat(value.floatValue, 'g', -1, 64)
	case FilterValueTimestamp:
		return "timestamp(" + strconv.Quote(value.timestamp.Format(time.RFC3339Nano)) + ")"
	case FilterValueDuration:
		return "duration(" + strconv.Quote(formatProtoDuration(value.duration)) + ")"
	default:
		return ""
	}
}

func formatProtoDuration(value durationScalar) string {
	negative := value.seconds < 0 || value.nanos < 0
	seconds := value.seconds
	nanos := value.nanos
	if negative {
		seconds = -seconds
		nanos = -nanos
	}
	prefix := ""
	if negative {
		prefix = "-"
	}
	if nanos == 0 {
		return prefix + strconv.FormatInt(seconds, 10) + "s"
	}
	return fmt.Sprintf("%s%d.%09ds", prefix, seconds, nanos)
}

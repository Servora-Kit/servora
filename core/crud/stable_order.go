package crud

import (
	"fmt"
	"slices"
	"strings"
)

// LogicalType identifies the canonical Proto comparison and cursor type.
type LogicalType string

const (
	LogicalString    LogicalType = "string"
	LogicalBool      LogicalType = "bool"
	LogicalEnum      LogicalType = "enum"
	LogicalInt32     LogicalType = "int32"
	LogicalInt64     LogicalType = "int64"
	LogicalUint32    LogicalType = "uint32"
	LogicalUint64    LogicalType = "uint64"
	LogicalFloat32   LogicalType = "float32"
	LogicalFloat64   LogicalType = "float64"
	LogicalBytes     LogicalType = "bytes"
	LogicalTimestamp LogicalType = "timestamp"
	LogicalDuration  LogicalType = "duration"
)

// OrderBinding identifies one stable backend sort key without exposing backend APIs.
type OrderBinding struct {
	key         string
	fieldPath   string
	nullable    bool
	profileID   string
	logicalType LogicalType
}

// NewOrderBinding constructs one immutable backend-neutral order binding.
func NewOrderBinding(key, fieldPath string, nullable bool, profileID string) (OrderBinding, error) {
	return NewTypedOrderBinding(key, fieldPath, nullable, profileID, LogicalString)
}

// NewTypedOrderBinding constructs one immutable binding with an explicit cursor logical type.
func NewTypedOrderBinding(
	key, fieldPath string,
	nullable bool,
	profileID string,
	logicalType LogicalType,
) (OrderBinding, error) {
	if strings.TrimSpace(key) == "" {
		return OrderBinding{}, fmt.Errorf("crud: order binding key is empty")
	}
	if strings.TrimSpace(profileID) == "" {
		return OrderBinding{}, fmt.Errorf("crud: order binding %q comparison profile is empty", key)
	}
	if !logicalType.valid() {
		return OrderBinding{}, fmt.Errorf("crud: order binding %q logical type %q is invalid", key, logicalType)
	}
	return OrderBinding{
		key: key, fieldPath: fieldPath, nullable: nullable, profileID: profileID, logicalType: logicalType,
	}, nil
}

// Key returns the adapter-local stable binding identity.
func (binding OrderBinding) Key() string { return binding.key }

// FieldPath returns the public resource field path, or empty for an internal key.
func (binding OrderBinding) FieldPath() string { return binding.fieldPath }

// Nullable reports whether the backend value can be NULL.
func (binding OrderBinding) Nullable() bool { return binding.nullable }

// ProfileID returns the stable, versioned comparison profile identifier.
func (binding OrderBinding) ProfileID() string { return binding.profileID }

// LogicalType returns the canonical cursor type.
func (binding OrderBinding) LogicalType() LogicalType { return binding.logicalType }

// ConfiguredOrderTerm is one repository-declared default or cursor-key term.
type ConfiguredOrderTerm struct {
	binding   OrderBinding
	direction OrderDirection
}

// NewConfiguredOrderTerm pairs a binding with its repository-declared direction.
func NewConfiguredOrderTerm(binding OrderBinding, direction OrderDirection) ConfiguredOrderTerm {
	return ConfiguredOrderTerm{binding: binding, direction: direction}
}

// OrderBindingResolver maps public resource fields to backend sort bindings.
type OrderBindingResolver interface {
	ResolveOrderBinding(FieldPlan) (OrderBinding, bool)
}

// OrderAssembler resolves client order terms and appends a complete unique cursor key.
type OrderAssembler struct {
	resolver   OrderBindingResolver
	defaults   []ConfiguredOrderTerm
	cursorKeys []ConfiguredOrderTerm
}

// NewOrderAssembler validates immutable repository order configuration.
func NewOrderAssembler(
	resolver OrderBindingResolver,
	defaults []ConfiguredOrderTerm,
	cursorKeys []ConfiguredOrderTerm,
) (*OrderAssembler, error) {
	if isNilInterface(resolver) {
		return nil, fmt.Errorf("crud: order binding resolver is nil")
	}
	if len(defaults) == 0 {
		return nil, fmt.Errorf("crud: default order is empty")
	}
	if len(cursorKeys) == 0 {
		return nil, fmt.Errorf("crud: cursor key is empty")
	}
	if err := validateConfiguredOrder("default order", defaults, false); err != nil {
		return nil, err
	}
	if err := validateConfiguredOrder("cursor key", cursorKeys, true); err != nil {
		return nil, err
	}
	defaultByKey := make(map[string]ConfiguredOrderTerm, len(defaults))
	for _, term := range defaults {
		defaultByKey[term.binding.key] = term
	}
	for _, cursor := range cursorKeys {
		if existing, ok := defaultByKey[cursor.binding.key]; ok &&
			(!sameOrderBinding(existing.binding, cursor.binding) || existing.direction != cursor.direction) {
			return nil, fmt.Errorf("crud: default order binding %q conflicts with cursor key contract", cursor.binding.key)
		}
	}
	return &OrderAssembler{
		resolver:   resolver,
		defaults:   slices.Clone(defaults),
		cursorKeys: slices.Clone(cursorKeys),
	}, nil
}

// FinalOrder is the immutable complete sort order used by adapters and tokens.
type FinalOrder struct {
	terms []FinalOrderTerm
}

// FinalOrderTerm contains one resolved backend binding and comparison contract.
type FinalOrderTerm struct {
	binding   OrderBinding
	direction OrderDirection
}

// Terms returns a copy of the complete order terms.
func (order FinalOrder) Terms() []FinalOrderTerm { return slices.Clone(order.terms) }

// Binding returns the resolved backend-neutral binding.
func (term FinalOrderTerm) Binding() OrderBinding { return term.binding }

// Direction returns the final direction.
func (term FinalOrderTerm) Direction() OrderDirection { return term.direction }

// NullsLast reports the fixed nullable comparison rule.
func (term FinalOrderTerm) NullsLast() bool { return term.binding.nullable }

// Resolve applies client order or the repository default, then appends missing cursor-key terms.
func (assembler *OrderAssembler) Resolve(client OrderExpression) (FinalOrder, error) {
	terms := make([]FinalOrderTerm, 0, max(len(client.terms), len(assembler.defaults))+len(assembler.cursorKeys))
	seen := make(map[string]OrderBinding, cap(terms))
	if client.Empty() {
		for _, configured := range assembler.defaults {
			if err := appendFinalOrderTerm(&terms, seen, configured.binding, configured.direction); err != nil {
				return FinalOrder{}, internalError("order_by", "%v", err)
			}
		}
	} else {
		for _, clientTerm := range client.terms {
			binding, ok := assembler.resolver.ResolveOrderBinding(clientTerm.field)
			if !ok {
				return FinalOrder{}, invalidOrderBy("order_by", "field %q is not enabled by the repository", clientTerm.field.path)
			}
			if err := appendFinalOrderTerm(&terms, seen, binding, clientTerm.direction); err != nil {
				return FinalOrder{}, invalidOrderBy("order_by", "%v", err)
			}
		}
	}
	for _, cursor := range assembler.cursorKeys {
		if existing, exists := seen[cursor.binding.key]; exists {
			if !sameOrderBinding(existing, cursor.binding) {
				return FinalOrder{}, internalError("order_by", "binding %q conflicts with cursor key contract", cursor.binding.key)
			}
			continue
		}
		if err := appendFinalOrderTerm(&terms, seen, cursor.binding, cursor.direction); err != nil {
			return FinalOrder{}, internalError("order_by", "%v", err)
		}
	}
	return FinalOrder{terms: terms}, nil
}

func validateConfiguredOrder(name string, terms []ConfiguredOrderTerm, requireNonNull bool) error {
	seen := make(map[string]struct{}, len(terms))
	for index, term := range terms {
		if err := validateOrderBinding(term.binding); err != nil {
			return fmt.Errorf("crud: %s term %d: %w", name, index, err)
		}
		if term.direction != OrderAscending && term.direction != OrderDescending {
			return fmt.Errorf("crud: %s term %d has invalid direction", name, index)
		}
		if requireNonNull && term.binding.nullable {
			return fmt.Errorf("crud: cursor key %q is nullable", term.binding.key)
		}
		if _, duplicate := seen[term.binding.key]; duplicate {
			return fmt.Errorf("crud: %s repeats binding %q", name, term.binding.key)
		}
		seen[term.binding.key] = struct{}{}
	}
	return nil
}

func validateOrderBinding(binding OrderBinding) error {
	if strings.TrimSpace(binding.key) == "" {
		return fmt.Errorf("order binding key is empty")
	}
	if strings.TrimSpace(binding.profileID) == "" {
		return fmt.Errorf("order binding %q comparison profile is empty", binding.key)
	}
	if !binding.logicalType.valid() {
		return fmt.Errorf("order binding %q logical type %q is invalid", binding.key, binding.logicalType)
	}
	return nil
}

func (logicalType LogicalType) valid() bool {
	switch logicalType {
	case LogicalString, LogicalBool, LogicalEnum, LogicalInt32, LogicalInt64,
		LogicalUint32, LogicalUint64, LogicalFloat32, LogicalFloat64, LogicalBytes,
		LogicalTimestamp, LogicalDuration:
		return true
	default:
		return false
	}
}

func appendFinalOrderTerm(
	terms *[]FinalOrderTerm,
	seen map[string]OrderBinding,
	binding OrderBinding,
	direction OrderDirection,
) error {
	if err := validateOrderBinding(binding); err != nil {
		return err
	}
	if direction != OrderAscending && direction != OrderDescending {
		return fmt.Errorf("binding %q has invalid direction", binding.key)
	}
	if _, duplicate := seen[binding.key]; duplicate {
		return fmt.Errorf("multiple order fields map to binding %q", binding.key)
	}
	seen[binding.key] = binding
	*terms = append(*terms, FinalOrderTerm{binding: binding, direction: direction})
	return nil
}

func sameOrderBinding(left, right OrderBinding) bool {
	return left == right
}

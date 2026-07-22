package crud

import (
	"fmt"
	"slices"
	"strings"

	annotations "google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// OrderDirection identifies ascending or descending sort order.
type OrderDirection uint8

const (
	OrderAscending OrderDirection = iota
	OrderDescending
)

// OrderExpression is an immutable normalized client order_by expression.
type OrderExpression struct {
	terms      []OrderTerm
	normalized string
}

// OrderTerm is one descriptor-resolved order expression term.
type OrderTerm struct {
	field     FieldPlan
	direction OrderDirection
}

// Empty reports whether the client omitted order_by.
func (order OrderExpression) Empty() bool { return len(order.terms) == 0 }

// String returns canonical AIP-132 text. Ascending directions are omitted.
func (order OrderExpression) String() string { return order.normalized }

// TermCount returns the number of client order terms.
func (order OrderExpression) TermCount() int { return len(order.terms) }

// Terms returns a copy of the client order terms.
func (order OrderExpression) Terms() []OrderTerm { return slices.Clone(order.terms) }

// Field returns the resource field referenced by this term.
func (term OrderTerm) Field() FieldPlan { return term.field }

// Direction returns the term direction.
func (term OrderTerm) Direction() OrderDirection { return term.direction }

func parseOrderBy(value string, resource filterResource) (OrderExpression, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return OrderExpression{}, nil
	}
	parts := strings.Split(value, ",")
	terms := make([]OrderTerm, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		words := strings.Fields(part)
		if len(words) == 0 {
			return OrderExpression{}, fmt.Errorf("empty order term")
		}
		if len(words) > 2 {
			return OrderExpression{}, fmt.Errorf("order term %q has extra tokens", strings.TrimSpace(part))
		}
		path := words[0]
		field, ok := resource.Field(path)
		if !ok || field.HasBehavior(annotations.FieldBehavior_IDENTIFIER) || field.HasBehavior(annotations.FieldBehavior_INPUT_ONLY) {
			return OrderExpression{}, fmt.Errorf("field %q is not orderable", path)
		}
		if !isSortableField(field.descriptor) {
			return OrderExpression{}, fmt.Errorf("field %q has non-sortable type %s", path, field.descriptor.Kind())
		}
		if _, duplicate := seen[path]; duplicate {
			return OrderExpression{}, fmt.Errorf("field %q is repeated", path)
		}
		seen[path] = struct{}{}

		direction := OrderAscending
		if len(words) == 2 {
			switch words[1] {
			case "asc":
			case "desc":
				direction = OrderDescending
			default:
				return OrderExpression{}, fmt.Errorf("unknown direction %q", words[1])
			}
		}
		terms = append(terms, OrderTerm{field: field, direction: direction})
	}

	normalized := make([]string, len(terms))
	for index, term := range terms {
		normalized[index] = term.field.path
		if term.direction == OrderDescending {
			normalized[index] += " desc"
		}
	}
	return OrderExpression{terms: terms, normalized: strings.Join(normalized, ", ")}, nil
}

func isSortableField(field protoreflect.FieldDescriptor) bool {
	if field.IsList() || field.IsMap() {
		return false
	}
	switch field.Kind() {
	case protoreflect.BoolKind, protoreflect.EnumKind,
		protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind, protoreflect.StringKind:
		return true
	case protoreflect.MessageKind:
		return isTimestampField(field) || isDurationField(field)
	default:
		return false
	}
}

package scope

import (
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

// ByUUID returns a single-element predicate slice if id is a valid UUID,
// or nil otherwise. Designed for use with Ent's variadic .Where():
//
//	q.Where(scope.ByUUID(id, entity.FieldEQ)...)
//	preds = append(preds, scope.ByUUID(id, entity.FieldEQ)...)
func ByUUID[P ~func(*sql.Selector)](id string, eqFn func(uuid.UUID) P) []P {
	if uid, err := uuid.Parse(id); err == nil {
		return []P{eqFn(uid)}
	}
	return nil
}

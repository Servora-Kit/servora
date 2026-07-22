package mixin

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

const (
	DeleteTimeField = "delete_time"
	DeletedByField  = "deleted_by"
	PurgeTimeField  = "purge_time"
)

type skipSoftDeleteKey struct{}
type deletedByKey struct{}

// SkipSoftDelete bypasses the default tombstone filter and converts no delete
// mutations. Callers use it explicitly for tombstone reads and hard deletes.
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, skipSoftDeleteKey{}, true)
}

// WithDeletedBy attaches the canonical actor name stored by a soft delete.
func WithDeletedBy(parent context.Context, actor string) context.Context {
	return context.WithValue(parent, deletedByKey{}, actor)
}

// SoftDeleteMixin provides tombstone fields, default query filtering, and
// Delete/DeleteOne-to-Update mutation rewriting for Ent schemas.
type SoftDeleteMixin struct {
	mixin.Schema
	now func() time.Time
}

func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time(DeleteTimeField).Optional().Nillable(),
		field.String(DeletedByField).Optional().Nillable(),
		field.Time(PurgeTimeField).Optional().Nillable(),
	}
}

func (SoftDeleteMixin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields(DeleteTimeField),
		index.Fields(PurgeTimeField),
	}
}

func (mixin SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		ent.TraverseFunc(func(ctx context.Context, query ent.Query) error {
			if skipsSoftDelete(ctx) {
				return nil
			}
			if err := mixin.addActiveQueryPredicate(query); err != nil {
				return err
			}
			return nil
		}),
	}
}

func (mixin SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
				if !mutation.Op().Is(ent.OpDeleteOne|ent.OpDelete) || skipsSoftDelete(ctx) {
					return next.Mutate(ctx, mutation)
				}
				rewrite, ok := mutation.(softDeleteMutation)
				if !ok {
					return nil, fmt.Errorf("soft delete mutation %T does not support operation rewrite", mutation)
				}
				mixin.addActivePredicate(rewrite)
				deletedAt := time.Now().UTC()
				if mixin.now != nil {
					deletedAt = mixin.now()
				}
				if err := mutation.SetField(DeleteTimeField, deletedAt); err != nil {
					return nil, fmt.Errorf("set %s: %w", DeleteTimeField, err)
				}
				if actor, ok := ctx.Value(deletedByKey{}).(string); ok && actor != "" {
					if err := mutation.SetField(DeletedByField, actor); err != nil {
						return nil, fmt.Errorf("set %s: %w", DeletedByField, err)
					}
				}
				rewrite.SetOp(ent.OpUpdate)
				return routeMutation(ctx, mutation)
			})
		},
	}
}

type wherePredicates interface {
	WhereP(...func(*sql.Selector))
}

type softDeleteMutation interface {
	ent.Mutation
	wherePredicates
	SetOp(ent.Op)
}

func (SoftDeleteMixin) addActivePredicate(target wherePredicates) {
	target.WhereP(sql.FieldIsNull(DeleteTimeField))
}

func (SoftDeleteMixin) addActiveQueryPredicate(query ent.Query) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("soft delete query %T modifier failed: %v", query, recovered)
		}
	}()
	method := reflect.ValueOf(query).MethodByName("Modify")
	modifierType := reflect.TypeFor[func(*sql.Selector)]()
	if !method.IsValid() || !method.Type().IsVariadic() || method.Type().NumIn() != 1 || method.Type().In(0).Elem() != modifierType {
		return fmt.Errorf("soft delete query %T does not expose generated SQL modifiers", query)
	}
	modifier := func(selector *sql.Selector) {
		selector.Where(sql.IsNull(selector.C(DeleteTimeField)))
	}
	method.Call([]reflect.Value{reflect.ValueOf(modifier)})
	return nil
}

func skipsSoftDelete(ctx context.Context) bool {
	skip, _ := ctx.Value(skipSoftDeleteKey{}).(bool)
	return skip
}

// routeMutation re-enters the generated client mutation router after changing
// Delete/DeleteOne to Update. Ent's generated Client method has a package-
// specific return type, so a reusable external mixin cannot express it as a Go
// interface; reflection is isolated here and fails closed on shape drift.
func routeMutation(ctx context.Context, mutation ent.Mutation) (value ent.Value, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value = nil
			err = fmt.Errorf("route rewritten soft delete mutation: %v", recovered)
		}
	}()
	clientMethod := reflect.ValueOf(mutation).MethodByName("Client")
	if !clientMethod.IsValid() || clientMethod.Type().NumIn() != 0 || clientMethod.Type().NumOut() != 1 {
		return nil, fmt.Errorf("soft delete mutation %T has no generated Client method", mutation)
	}
	client := clientMethod.Call(nil)[0]
	if (client.Kind() == reflect.Pointer || client.Kind() == reflect.Interface) && client.IsNil() {
		return nil, fmt.Errorf("soft delete mutation %T returned a nil client", mutation)
	}
	mutate := client.MethodByName("Mutate")
	if !mutate.IsValid() {
		return nil, fmt.Errorf("soft delete mutation client %T has no Mutate method", client.Interface())
	}
	results := mutate.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(mutation)})
	if len(results) != 2 {
		return nil, fmt.Errorf("soft delete mutation client %T returned %d values", client.Interface(), len(results))
	}
	if !results[1].IsNil() {
		mutationErr, ok := results[1].Interface().(error)
		if !ok {
			return nil, fmt.Errorf("soft delete mutation client %T returned a non-error result", client.Interface())
		}
		return nil, mutationErr
	}
	return results[0].Interface(), nil
}

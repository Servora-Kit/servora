package mixin

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
)

func TestSoftDeleteMixinSchema(t *testing.T) {
	t.Parallel()

	mixin := SoftDeleteMixin{}
	fields := mixin.Fields()
	if len(fields) != 3 {
		t.Fatalf("Fields() length = %d, want 3", len(fields))
	}
	for index, name := range []string{DeleteTimeField, DeletedByField, PurgeTimeField} {
		descriptor := fields[index].Descriptor()
		if descriptor.Name != name {
			t.Errorf("field %d name = %q, want %q", index, descriptor.Name, name)
		}
		if !descriptor.Optional || !descriptor.Nillable {
			t.Errorf("field %s must be optional and nillable", name)
		}
	}
	indexes := mixin.Indexes()
	if len(indexes) != 2 {
		t.Fatalf("Indexes() length = %d, want 2", len(indexes))
	}
}

func TestSoftDeleteMixinFiltersQueriesUnlessBypassed(t *testing.T) {
	t.Parallel()

	mixin := SoftDeleteMixin{}
	interceptors := mixin.Interceptors()
	if len(interceptors) != 1 {
		t.Fatalf("Interceptors() length = %d, want 1", len(interceptors))
	}
	traverser, ok := interceptors[0].(ent.Traverser)
	if !ok {
		t.Fatalf("interceptor %T is not an ent.Traverser", interceptors[0])
	}

	query := &fakeWhereQuery{}
	if err := traverser.Traverse(context.Background(), query); err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	if len(query.predicates) != 1 {
		t.Fatalf("query predicates = %d, want 1", len(query.predicates))
	}
	bypassed := &fakeWhereQuery{}
	if err := traverser.Traverse(SkipSoftDelete(context.Background()), bypassed); err != nil {
		t.Fatalf("Traverse bypassed: %v", err)
	}
	if len(bypassed.predicates) != 0 {
		t.Fatalf("bypassed query predicates = %d, want 0", len(bypassed.predicates))
	}
}

func TestSoftDeleteMixinConvertsDeleteToUpdate(t *testing.T) {
	t.Parallel()

	deletedAt := time.Date(2026, time.July, 21, 13, 0, 0, 0, time.UTC)
	client := &fakeMutationClient{result: 1}
	mutation := newFakeMutation(ent.OpDelete, client)
	mixin := SoftDeleteMixin{now: func() time.Time { return deletedAt }}
	hooks := mixin.Hooks()
	if len(hooks) != 1 {
		t.Fatalf("Hooks() length = %d, want 1", len(hooks))
	}
	nextCalled := false
	next := ent.MutateFunc(func(context.Context, ent.Mutation) (ent.Value, error) {
		nextCalled = true
		return nil, nil
	})

	value, err := hooks[0](next).Mutate(WithDeletedBy(context.Background(), "users/admin"), mutation)
	if err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if value != 1 {
		t.Fatalf("soft delete value = %v, want 1", value)
	}
	if nextCalled {
		t.Fatal("delete-specific mutator was called after operation rewrite")
	}
	if !client.called {
		t.Fatal("generated client mutation router was not called")
	}
	if mutation.Op() != ent.OpUpdate {
		t.Fatalf("mutation op = %s, want update", mutation.Op())
	}
	if got, ok := mutation.Field(DeleteTimeField); !ok || got != deletedAt {
		t.Fatalf("delete_time = %v, %v", got, ok)
	}
	if got, ok := mutation.Field(DeletedByField); !ok || got != "users/admin" {
		t.Fatalf("deleted_by = %v, %v", got, ok)
	}
	if len(mutation.predicates) != 1 {
		t.Fatalf("mutation predicates = %d, want 1", len(mutation.predicates))
	}
}

func TestSoftDeleteMixinBypassPreservesHardDelete(t *testing.T) {
	t.Parallel()

	client := &fakeMutationClient{}
	mutation := newFakeMutation(ent.OpDeleteOne, client)
	mixin := SoftDeleteMixin{}
	nextCalled := false
	next := ent.MutateFunc(func(_ context.Context, got ent.Mutation) (ent.Value, error) {
		nextCalled = true
		if got.Op() != ent.OpDeleteOne {
			t.Fatalf("bypassed op = %s, want delete-one", got.Op())
		}
		return 1, nil
	})

	if _, err := mixin.Hooks()[0](next).Mutate(SkipSoftDelete(context.Background()), mutation); err != nil {
		t.Fatalf("hard delete bypass: %v", err)
	}
	if !nextCalled {
		t.Fatal("hard delete did not call next mutator")
	}
	if client.called {
		t.Fatal("hard delete unexpectedly used mutation rerouter")
	}
	if len(mutation.Fields()) != 0 || len(mutation.predicates) != 0 {
		t.Fatal("hard delete mutation was modified")
	}
}

type fakeWhereQuery struct {
	predicates []func(*sql.Selector)
}

func (query *fakeWhereQuery) Modify(predicates ...func(*sql.Selector)) *fakeWhereQuery {
	query.predicates = append(query.predicates, predicates...)
	return query
}

type fakeMutationClient struct {
	called bool
	result ent.Value
}

func (client *fakeMutationClient) Mutate(_ context.Context, _ ent.Mutation) (ent.Value, error) {
	client.called = true
	return client.result, nil
}

type fakeMutation struct {
	op         ent.Op
	client     *fakeMutationClient
	fields     map[string]ent.Value
	cleared    map[string]bool
	predicates []func(*sql.Selector)
}

func newFakeMutation(op ent.Op, client *fakeMutationClient) *fakeMutation {
	return &fakeMutation{op: op, client: client, fields: make(map[string]ent.Value), cleared: make(map[string]bool)}
}

func (mutation *fakeMutation) Client() *fakeMutationClient { return mutation.client }
func (mutation *fakeMutation) Op() ent.Op                  { return mutation.op }
func (mutation *fakeMutation) SetOp(op ent.Op)             { mutation.op = op }
func (mutation *fakeMutation) Type() string                { return "Fake" }
func (mutation *fakeMutation) Fields() []string {
	fields := make([]string, 0, len(mutation.fields))
	for name := range mutation.fields {
		fields = append(fields, name)
	}
	return fields
}
func (mutation *fakeMutation) Field(name string) (ent.Value, bool) {
	value, ok := mutation.fields[name]
	return value, ok
}
func (mutation *fakeMutation) SetField(name string, value ent.Value) error {
	mutation.fields[name] = value
	return nil
}
func (*fakeMutation) AddedFields() []string               { return nil }
func (*fakeMutation) AddedField(string) (ent.Value, bool) { return nil, false }
func (*fakeMutation) AddField(string, ent.Value) error    { return nil }
func (mutation *fakeMutation) ClearedFields() []string {
	fields := make([]string, 0, len(mutation.cleared))
	for name := range mutation.cleared {
		fields = append(fields, name)
	}
	return fields
}
func (mutation *fakeMutation) FieldCleared(name string) bool { return mutation.cleared[name] }
func (mutation *fakeMutation) ClearField(name string) error {
	mutation.cleared[name] = true
	return nil
}
func (mutation *fakeMutation) ResetField(name string) error {
	delete(mutation.fields, name)
	delete(mutation.cleared, name)
	return nil
}
func (*fakeMutation) AddedEdges() []string                                { return nil }
func (*fakeMutation) AddedIDs(string) []ent.Value                         { return nil }
func (*fakeMutation) RemovedEdges() []string                              { return nil }
func (*fakeMutation) RemovedIDs(string) []ent.Value                       { return nil }
func (*fakeMutation) ClearedEdges() []string                              { return nil }
func (*fakeMutation) EdgeCleared(string) bool                             { return false }
func (*fakeMutation) ClearEdge(string) error                              { return nil }
func (*fakeMutation) ResetEdge(string) error                              { return nil }
func (*fakeMutation) OldField(context.Context, string) (ent.Value, error) { return nil, nil }
func (mutation *fakeMutation) WhereP(predicates ...func(*sql.Selector)) {
	mutation.predicates = append(mutation.predicates, predicates...)
}

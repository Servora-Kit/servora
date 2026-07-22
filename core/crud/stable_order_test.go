package crud_test

import (
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
)

type testOrderResolver map[string]crud.OrderBinding

func (resolver testOrderResolver) ResolveOrderBinding(field crud.FieldPlan) (crud.OrderBinding, bool) {
	binding, ok := resolver[field.Path()]
	return binding, ok
}

type pointerOrderResolver struct{}

func (*pointerOrderResolver) ResolveOrderBinding(crud.FieldPlan) (crud.OrderBinding, bool) {
	return crud.OrderBinding{}, false
}

func TestOrderAssemblerAppendsUniqueCursorKey(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	resolver := testOrderResolver{
		"display_name": mustOrderBinding(t, "display_name", "display_name", false, "proto/string:utf8-binary-v1"),
		"nickname":     mustOrderBinding(t, "nickname", "nickname", true, "proto/string:utf8-binary-v1"),
		"create_time":  mustOrderBinding(t, "created_at", "create_time", false, "proto/timestamp:v1"),
	}
	cursor := mustOrderBinding(t, "id", "", false, "proto/int64:v1")
	assembler, err := crud.NewOrderAssembler(
		resolver,
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(resolver["create_time"], crud.OrderDescending)},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
	)
	if err != nil {
		t.Fatalf("NewOrderAssembler: %v", err)
	}

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	query, err := preparer.PrepareList(plan, crud.ListInput{OrderBy: "nickname desc"})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	finalOrder, err := assembler.Resolve(query.OrderBy())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	terms := finalOrder.Terms()
	if got, want := len(terms), 2; got != want {
		t.Fatalf("len(Terms()) = %d, want %d", got, want)
	}
	if got, want := terms[0].Binding().Key(), "nickname"; got != want {
		t.Fatalf("first key = %q, want %q", got, want)
	}
	if !terms[0].NullsLast() {
		t.Fatal("nullable client order does not force NULLS LAST")
	}
	if got, want := terms[1].Binding().Key(), "id"; got != want {
		t.Fatalf("cursor key = %q, want %q", got, want)
	}
}

func TestOrderAssemblerUsesCompleteDefaultOrder(t *testing.T) {
	t.Parallel()

	created := mustOrderBinding(t, "created_at", "create_time", false, "proto/timestamp:v1")
	cursor := mustOrderBinding(t, "id", "", false, "proto/int64:v1")
	assembler, err := crud.NewOrderAssembler(
		testOrderResolver{},
		[]crud.ConfiguredOrderTerm{
			crud.NewConfiguredOrderTerm(created, crud.OrderDescending),
			crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending),
		},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
	)
	if err != nil {
		t.Fatalf("NewOrderAssembler: %v", err)
	}
	finalOrder, err := assembler.Resolve(crud.OrderExpression{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got, want := len(finalOrder.Terms()), 2; got != want {
		t.Fatalf("len(Terms()) = %d, want %d", got, want)
	}
}

func TestOrderAssemblerRejectsUnopenedOrderField(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	cursor := mustOrderBinding(t, "id", "", false, "proto/int64:v1")
	assembler, err := crud.NewOrderAssembler(
		testOrderResolver{},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
	)
	if err != nil {
		t.Fatalf("NewOrderAssembler: %v", err)
	}
	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	query, err := preparer.PrepareList(plan, crud.ListInput{OrderBy: "display_name"})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	_, err = assembler.Resolve(query.OrderBy())
	if !crudpb.IsCrudErrorReasonInvalidOrderBy(err) {
		t.Fatalf("Resolve error = %v, want INVALID_ORDER_BY", err)
	}
}

func TestOrderAssemblerRejectsNullableCursorKey(t *testing.T) {
	t.Parallel()

	nullable := mustOrderBinding(t, "id", "", true, "proto/int64:v1")
	_, err := crud.NewOrderAssembler(
		testOrderResolver{},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(nullable, crud.OrderAscending)},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(nullable, crud.OrderAscending)},
	)
	if err == nil {
		t.Fatal("NewOrderAssembler accepted nullable cursor key")
	}
}

func TestOrderAssemblerRejectsConflictingDefaultCursorContract(t *testing.T) {
	t.Parallel()

	defaultBinding := mustTypedOrderBinding(t, "id", "", false, "proto/string:utf8-binary-v1", crud.LogicalString)
	cursorBinding := mustTypedOrderBinding(t, "id", "", false, "proto/uint64:v1", crud.LogicalUint64)
	_, err := crud.NewOrderAssembler(
		testOrderResolver{},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(defaultBinding, crud.OrderAscending)},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursorBinding, crud.OrderAscending)},
	)
	if err == nil {
		t.Fatal("NewOrderAssembler accepted conflicting same-key contracts")
	}
}

func TestOrderAssemblerRejectsTypedNilResolver(t *testing.T) {
	t.Parallel()

	cursor := mustTypedOrderBinding(t, "id", "", false, "proto/uint64:v1", crud.LogicalUint64)
	var resolver *pointerOrderResolver
	if _, err := crud.NewOrderAssembler(
		resolver,
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
	); err == nil {
		t.Fatal("NewOrderAssembler accepted typed-nil resolver")
	}
}

func TestOrderAssemblerRejectsClientBindingThatConflictsWithCursorKey(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	cursor := mustTypedOrderBinding(t, "id", "", false, "proto/uint64:v1", crud.LogicalUint64)
	conflicting := mustTypedOrderBinding(t, "id", "display_name", false, "proto/string:utf8-binary-v1", crud.LogicalString)
	assembler, err := crud.NewOrderAssembler(
		testOrderResolver{"display_name": conflicting},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
		[]crud.ConfiguredOrderTerm{crud.NewConfiguredOrderTerm(cursor, crud.OrderAscending)},
	)
	if err != nil {
		t.Fatalf("NewOrderAssembler: %v", err)
	}
	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	query, err := preparer.PrepareList(plan, crud.ListInput{OrderBy: "display_name"})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	if _, err := assembler.Resolve(query.OrderBy()); err == nil {
		t.Fatal("Resolve accepted conflicting client/cursor binding")
	}
}

func mustOrderBinding(t *testing.T, key, path string, nullable bool, profile string) crud.OrderBinding {
	t.Helper()
	binding, err := crud.NewOrderBinding(key, path, nullable, profile)
	if err != nil {
		t.Fatalf("NewOrderBinding: %v", err)
	}
	return binding
}

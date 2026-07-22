package crud

import (
	"strings"
	"testing"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	corecrud "github.com/Servora-Kit/servora/core/crud"
)

func TestNewListFieldsBuildsImmutableOrderContract(t *testing.T) {
	t.Parallel()

	fields, err := NewListFields[struct{}](
		Columns(testValidColumn("created_at", "nickname", "id")),
		Bind(examplev1.UserFields.CreateTime, "created_at").Filter().Order(),
		Bind(examplev1.UserFields.Nickname, "nickname").Filter().Order().Nullable(),
		DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderDescending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}

	plan := corecrud.MustBuildResourcePlan[*examplev1.User](examplev1.UserCRUDDescriptor())
	createTime, ok := plan.Field("create_time")
	if !ok {
		t.Fatal("resource plan has no create_time field")
	}
	binding, ok := fields.ResolveOrderBinding(createTime)
	if !ok {
		t.Fatal("create_time order binding is absent")
	}
	if got, want := binding.Key(), "created_at"; got != want {
		t.Fatalf("binding key = %q, want %q", got, want)
	}
	if got, want := binding.LogicalType(), corecrud.LogicalTimestamp; got != want {
		t.Fatalf("logical type = %q, want %q", got, want)
	}
	if got, want := binding.ProfileID(), "proto/timestamp:v1"; got != want {
		t.Fatalf("profile = %q, want %q", got, want)
	}

	finalOrder, err := fields.orderAssembler.Resolve(corecrud.OrderExpression{})
	if err != nil {
		t.Fatalf("Resolve default order: %v", err)
	}
	terms := finalOrder.Terms()
	if got, want := len(terms), 2; got != want {
		t.Fatalf("final order term count = %d, want %d", got, want)
	}
	if got, want := terms[0].Binding().Key(), "created_at"; got != want {
		t.Fatalf("first term = %q, want %q", got, want)
	}
	if got, want := terms[0].Direction(), corecrud.OrderDescending; got != want {
		t.Fatalf("first direction = %v, want %v", got, want)
	}
	if got, want := terms[1].Binding().Key(), "id"; got != want {
		t.Fatalf("cursor term = %q, want %q", got, want)
	}
}

func TestNewListFieldsRejectsInvalidStaticConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options []ListFieldsOption
		want    string
	}{
		{
			name: "missing columns",
			options: []ListFieldsOption{
				Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
				DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
				CursorKey[uint32]("id", corecrud.OrderAscending),
			},
			want: "Columns is required",
		},
		{
			name: "unknown bound column",
			options: []ListFieldsOption{
				Columns(testValidColumn("id")),
				Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
				DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
				CursorKey[uint32]("id", corecrud.OrderAscending),
			},
			want: "not generated for this table",
		},
		{
			name: "missing default order",
			options: []ListFieldsOption{
				Columns(testValidColumn("created_at", "id")),
				Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
				CursorKey[uint32]("id", corecrud.OrderAscending),
			},
			want: "default order is empty",
		},
		{
			name: "missing cursor key",
			options: []ListFieldsOption{
				Columns(testValidColumn("created_at")),
				Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
				DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
			},
			want: "cursor key is empty",
		},
		{
			name: "default lacks order capability",
			options: []ListFieldsOption{
				Columns(testValidColumn("created_at", "id")),
				Bind(examplev1.UserFields.CreateTime, "created_at").Filter(),
				DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
				CursorKey[uint32]("id", corecrud.OrderAscending),
			},
			want: "has no Order capability",
		},
		{
			name: "duplicate field binding",
			options: []ListFieldsOption{
				Columns(testValidColumn("created_at", "id")),
				Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
				Bind(examplev1.UserFields.CreateTime, "created_at").Filter(),
				DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
				CursorKey[uint32]("id", corecrud.OrderAscending),
			},
			want: "bound more than once",
		},
		{
			name: "unsupported cursor type",
			options: []ListFieldsOption{
				Columns(testValidColumn("created_at", "id")),
				Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
				DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderAscending),
				CursorKey[complex64]("id", corecrud.OrderAscending),
			},
			want: "cursor key Go type complex64 is unsupported",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewListFields[struct{}](test.options...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewListFields error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestBindingFluentMethodsReturnCopies(t *testing.T) {
	t.Parallel()

	base := Bind(examplev1.UserFields.CreateTime, "created_at")
	filter := base.Filter()
	ordered := filter.Order()
	if base.filter || base.order {
		t.Fatal("base binding was mutated")
	}
	if !filter.filter || filter.order {
		t.Fatal("Filter did not return an isolated configured copy")
	}
	if !ordered.filter || !ordered.order {
		t.Fatal("Order did not preserve Filter on its returned copy")
	}
}

func testValidColumn(columns ...string) ValidColumn {
	allowed := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		allowed[column] = struct{}{}
	}
	return func(column string) bool {
		_, ok := allowed[column]
		return ok
	}
}

package crud

import (
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	corecrud "github.com/Servora-Kit/servora/core/crud"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestListFieldsCompilesTypedColumnFilter(t *testing.T) {
	t.Parallel()

	fields := mustListFields(t,
		Bind(examplev1.UserFields.Email, "email").Filter(),
		Bind(examplev1.UserFields.CreateTime, "created_at").Filter().Order(),
	)
	query := prepareListFilter(t, `email >= "m" AND create_time < "2026-02-01T00:00:00Z"`)
	selector := sql.Dialect(dialect.Postgres).Select("*").From(sql.Table("users"))
	predicate, err := fields.filterPredicate(selector, query.Filter())
	if err != nil {
		t.Fatalf("filterPredicate: %v", err)
	}
	selector.Where(predicate)
	statement, arguments := selector.Query()
	if !strings.Contains(statement, `"users"."email" COLLATE "C" >= $1`) {
		t.Fatalf("query does not enforce binary string comparison: %s", statement)
	}
	if !strings.Contains(statement, `"users"."created_at" < $2`) {
		t.Fatalf("query does not contain timestamp comparison: %s", statement)
	}
	if got, want := fmt.Sprint(arguments), `[m 2026-02-01 00:00:00 +0000 UTC]`; got != want {
		t.Fatalf("arguments = %s, want %s", got, want)
	}
}

func TestListFieldsRejectsUnboundFilterBeforeExecution(t *testing.T) {
	t.Parallel()

	fields := mustListFields(t, Bind(examplev1.UserFields.CreateTime, "created_at").Order())
	query := prepareListFilter(t, `email = "person@example.com"`)
	selector := sql.Dialect(dialect.SQLite).Select("*").From(sql.Table("users"))
	_, err := fields.filterPredicate(selector, query.Filter())
	if err == nil {
		t.Fatal("filterPredicate accepted an unbound field")
	}
	frameworkError, ok := err.(*kratoserrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if got, want := frameworkError.GetReason(), crudpb.CrudErrorReason_CRUD_ERROR_REASON_INVALID_FILTER.String(); got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
}

func TestJSONPathAndCustomBindings(t *testing.T) {
	t.Parallel()

	customCalled := false
	fields := mustListFields(t,
		JSONPath(examplev1.UserFields.Email, "profile", "contact", "email").Filter(),
		Custom(examplev1.UserFields.TenantPlan, "plan_rank", func(
			operator corecrud.FilterOperator,
			value corecrud.FilterValue,
		) (SelectorPredicate, error) {
			customCalled = true
			if operator != corecrud.FilterOperatorEqual {
				return nil, fmt.Errorf("unsupported operator %s", operator)
			}
			plan, ok := value.StringValue()
			if !ok {
				return nil, fmt.Errorf("expected string")
			}
			return func(selector *sql.Selector) *sql.Predicate {
				return sql.EQ(selector.C("plan_code"), strings.ToUpper(plan))
			}, nil
		}).Filter(),
		Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
	)
	query := prepareListFilter(t, `email = "person@example.com" AND tenant_plan = "pro"`)
	selector := sql.Dialect(dialect.SQLite).Select("*").From(sql.Table("users"))
	predicate, err := fields.filterPredicate(selector, query.Filter())
	if err != nil {
		t.Fatalf("filterPredicate: %v", err)
	}
	selector.Where(predicate)
	statement, arguments := selector.Query()
	if !customCalled {
		t.Fatal("custom predicate was not called")
	}
	if !strings.Contains(statement, "JSON_EXTRACT") || !strings.Contains(statement, "`users`.`plan_code` = ?") {
		t.Fatalf("query does not contain JSONPath and custom predicates: %s", statement)
	}
	if got, want := fmt.Sprint(arguments), `[person@example.com PRO]`; got != want {
		t.Fatalf("arguments = %s, want %s", got, want)
	}
}

func TestBindingAndCursorConverters(t *testing.T) {
	t.Parallel()

	fields, err := NewListFields[struct{}](
		Columns(testValidColumn("email_hash", "created_at", "id")),
		Bind(examplev1.UserFields.Email, "email_hash").Filter().WithQueryConverter(
			func(value corecrud.FilterValue) (any, error) {
				email, ok := value.StringValue()
				if !ok {
					return nil, fmt.Errorf("expected string")
				}
				return "hash:" + email, nil
			},
		),
		Bind(examplev1.UserFields.CreateTime, "created_at").Order(),
		DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderDescending),
		CursorKey[uint32]("id", corecrud.OrderAscending).WithConverter(func(value any) (uint32, error) {
			id, ok := value.(int64)
			if !ok || id < 0 {
				return 0, fmt.Errorf("invalid id")
			}
			return uint32(id), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}
	query := prepareListFilter(t, `email = "person@example.com"`)
	selector := sql.Dialect(dialect.SQLite).Select("*").From(sql.Table("users"))
	predicate, err := fields.filterPredicate(selector, query.Filter())
	if err != nil {
		t.Fatalf("filterPredicate: %v", err)
	}
	selector.Where(predicate)
	_, arguments := selector.Query()
	if got, want := fmt.Sprint(arguments), `[hash:person@example.com]`; got != want {
		t.Fatalf("arguments = %s, want %s", got, want)
	}
	converted, err := fields.cursorKeys[0].converter(int64(42))
	if err != nil || converted != uint32(42) {
		t.Fatalf("cursor converter = (%v, %v), want (42, nil)", converted, err)
	}
}

func mustListFields(t *testing.T, bindings ...Binding) *ListFields[struct{}] {
	t.Helper()
	options := []ListFieldsOption{Columns(testValidColumn(
		"email", "profile", "plan_rank", "plan_code", "created_at", "id",
	))}
	for _, binding := range bindings {
		options = append(options, binding)
	}
	options = append(options,
		DefaultOrder(examplev1.UserFields.CreateTime, corecrud.OrderDescending),
		CursorKey[uint32]("id", corecrud.OrderAscending),
	)
	fields, err := NewListFields[struct{}](options...)
	if err != nil {
		t.Fatalf("NewListFields: %v", err)
	}
	return fields
}

func prepareListFilter(t *testing.T, filter string) corecrud.ListQuery {
	t.Helper()
	preparer, err := corecrud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	plan := corecrud.MustBuildResourcePlan[*examplev1.User](examplev1.UserCRUDDescriptor())
	query, err := preparer.PrepareList(plan, corecrud.ListInput{
		Collection: "tenants/acme/users",
		Filter:     filter,
	})
	if err != nil {
		t.Fatalf("PrepareList(%q): %v", filter, err)
	}
	return query
}

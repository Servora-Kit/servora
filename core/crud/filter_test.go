package crud_test

import (
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
	"strings"
	"testing"
)

func TestPrepareListParsesTypedFilter(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)

	query, err := preparer.PrepareList(plan, crud.ListInput{
		Filter: `(display_name = "Alice" OR email != "blocked@example.com") AND create_time >= timestamp("2026-01-01T00:00:00Z")`,
	})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	filter := query.Filter()
	if filter.Empty() {
		t.Fatal("Filter is empty")
	}
	if got, want := filter.String(), `(display_name = "Alice" OR email != "blocked@example.com") AND create_time >= timestamp("2026-01-01T00:00:00Z")`; got != want {
		t.Fatalf("Filter.String() = %q, want %q", got, want)
	}
}

func TestPrepareListParsesNullFilterValue(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)

	query, err := preparer.PrepareList(plan, crud.ListInput{Filter: `nickname = null`})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	if got, want := query.Filter().Root().Value().Kind(), crud.FilterValueNull; got != want {
		t.Fatalf("filter value kind = %v, want %v", got, want)
	}
}

func TestPrepareListRejectsUnsupportedFilters(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)

	tests := []struct {
		name   string
		filter string
	}{
		{"bare literal", `display_name`},
		{"unary not", `NOT display_name = "Alice"`},
		{"string has", `display_name:"Ali"`},
		{"identifier field", `name = "tenants/acme/users/u-1"`},
		{"input-only field", `temporary_password = "secret"`},
		{"type mismatch", `display_name = 1`},
		{"invalid timestamp", `create_time = "not-a-time"`},
		{"custom function", `startsWith(display_name, "A")`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := preparer.PrepareList(plan, crud.ListInput{Filter: test.filter})
			if !crudpb.IsCrudErrorReasonInvalidFilter(err) {
				t.Fatalf("PrepareList error = %v, want INVALID_FILTER", err)
			}
		})
	}
}

func TestPrepareListEnforcesFilterLimits(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	tests := []struct {
		name   string
		limits crud.FilterLimits
		filter string
		text   string
	}{
		{"bytes", mustFilterLimits(t, crud.MaxFilterBytes(8)), `display_name = "Alice"`, "byte limit"},
		{"nodes", mustFilterLimits(t, crud.MaxFilterNodes(2)), `display_name = "Alice" AND email = "alice@example.com"`, "node limit"},
		{"depth", mustFilterLimits(t, crud.MaxFilterDepth(2)), `(((display_name = "Alice")))`, "depth limit"},
		{"OR terms", mustFilterLimits(t, crud.MaxFilterORTerms(2)), `display_name = "A" OR display_name = "B" OR display_name = "C"`, "OR term limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			preparer, err := crud.NewListPreparer(crud.WithResourceOverrides(
				plan.ResourceType(),
				crud.WithFilterLimits(test.limits),
			))
			if err != nil {
				t.Fatalf("NewListPreparer: %v", err)
			}
			_, err = preparer.PrepareList(plan, crud.ListInput{Filter: test.filter})
			if !crudpb.IsCrudErrorReasonInvalidFilter(err) || !strings.Contains(err.Error(), test.text) {
				t.Fatalf("PrepareList error = %v, want INVALID_FILTER containing %q", err, test.text)
			}
		})
	}
}

func TestPrepareListAllowsExplicitUnlimitedFilter(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	preparer, err := crud.NewListPreparer(crud.WithResourceOverrides(
		plan.ResourceType(),
		crud.WithFilterLimits(crud.UnlimitedFilterLimits()),
	))
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	value := strings.Repeat("x", 9*1024)
	if _, err := preparer.PrepareList(plan, crud.ListInput{Filter: `display_name = "` + value + `"`}); err != nil {
		t.Fatalf("PrepareList unlimited: %v", err)
	}
}

func mustFilterLimits(t *testing.T, options ...crud.FilterLimitOption) crud.FilterLimits {
	t.Helper()
	limits, err := crud.NewFilterLimits(options...)
	if err != nil {
		t.Fatalf("NewFilterLimits: %v", err)
	}
	return limits
}

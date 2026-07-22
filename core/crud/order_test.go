package crud_test

import (
	"strings"
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
)

func TestPrepareListParsesOrderBy(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)

	query, err := preparer.PrepareList(plan, crud.ListInput{OrderBy: " display_name   asc, create_time desc "})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	order := query.OrderBy()
	if got, want := order.String(), "display_name, create_time desc"; got != want {
		t.Fatalf("OrderBy.String() = %q, want %q", got, want)
	}
	terms := order.Terms()
	if got, want := len(terms), 2; got != want {
		t.Fatalf("len(Terms()) = %d, want %d", got, want)
	}
	if got, want := terms[0].Direction(), crud.OrderAscending; got != want {
		t.Fatalf("first direction = %v, want %v", got, want)
	}
	if got, want := terms[1].Direction(), crud.OrderDescending; got != want {
		t.Fatalf("second direction = %v, want %v", got, want)
	}
}

func TestPrepareListRejectsInvalidOrderBy(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	tests := []struct {
		name    string
		orderBy string
	}{
		{"dash direction", "-create_time"},
		{"unknown direction", "create_time ascending"},
		{"extra token", "create_time desc nulls last"},
		{"duplicate", "create_time, create_time desc"},
		{"unknown path", "missing"},
		{"identifier path", "name"},
		{"input-only path", "temporary_password"},
		{"empty term", "display_name,,email"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := preparer.PrepareList(plan, crud.ListInput{OrderBy: test.orderBy})
			if !crudpb.IsCrudErrorReasonInvalidOrderBy(err) {
				t.Fatalf("PrepareList error = %v, want INVALID_ORDER_BY", err)
			}
		})
	}
}

func TestPrepareListEnforcesOrderLimits(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	tests := []struct {
		name    string
		limits  crud.OrderLimits
		orderBy string
		text    string
	}{
		{"bytes", mustOrderLimits(t, crud.MaxOrderBytes(8)), "create_time", "byte limit"},
		{"terms", mustOrderLimits(t, crud.MaxOrderTerms(1)), "display_name, create_time", "term limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			preparer, err := crud.NewListPreparer(crud.WithResourceOverrides(
				plan.ResourceType(),
				crud.WithOrderLimits(test.limits),
			))
			if err != nil {
				t.Fatalf("NewListPreparer: %v", err)
			}
			_, err = preparer.PrepareList(plan, crud.ListInput{OrderBy: test.orderBy})
			if !crudpb.IsCrudErrorReasonInvalidOrderBy(err) || !strings.Contains(err.Error(), test.text) {
				t.Fatalf("PrepareList error = %v, want INVALID_ORDER_BY containing %q", err, test.text)
			}
		})
	}
}

func mustOrderLimits(t *testing.T, options ...crud.OrderLimitOption) crud.OrderLimits {
	t.Helper()
	limits, err := crud.NewOrderLimits(options...)
	if err != nil {
		t.Fatalf("NewOrderLimits: %v", err)
	}
	return limits
}

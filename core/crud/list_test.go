package crud_test

import (
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
)

type listResource string

func (resource listResource) ResourceType() string { return string(resource) }

func TestListPreparerConfigurationPrecedence(t *testing.T) {
	t.Parallel()

	applicationFilter, err := crud.NewFilterLimits(
		crud.MaxFilterBytes(4096),
		crud.MaxFilterNodes(64),
	)
	if err != nil {
		t.Fatalf("NewFilterLimits: %v", err)
	}
	preparer, err := crud.NewListPreparer(
		crud.WithApplicationDefaults(
			crud.WithFilterLimits(applicationFilter),
			crud.WithDefaultPageSize(100),
			crud.WithMaxPageSize(500),
		),
		crud.WithResourceOverrides(
			"example.servora.dev/User",
			crud.WithDefaultPageSize(25),
		),
	)
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}

	resource := preparer.SettingsFor("example.servora.dev/User")
	if got, want := resource.DefaultPageSize(), int32(25); got != want {
		t.Fatalf("resource DefaultPageSize = %d, want %d", got, want)
	}
	if got, limited := resource.MaxPageSize(); !limited || got != 500 {
		t.Fatalf("resource MaxPageSize = (%d, %v), want (500, true)", got, limited)
	}
	if got, limited := resource.FilterLimits().MaxBytes(); !limited || got != 4096 {
		t.Fatalf("resource filter MaxBytes = (%d, %v), want (4096, true)", got, limited)
	}
	if got, limited := resource.FilterLimits().MaxNodes(); !limited || got != 64 {
		t.Fatalf("resource filter MaxNodes = (%d, %v), want (64, true)", got, limited)
	}
	if got, limited := resource.FilterLimits().MaxDepth(); !limited || got != 8 {
		t.Fatalf("resource filter MaxDepth = (%d, %v), want framework default (8, true)", got, limited)
	}

	application := preparer.SettingsFor("another.servora.dev/Resource")
	if got, want := application.DefaultPageSize(), int32(100); got != want {
		t.Fatalf("application DefaultPageSize = %d, want %d", got, want)
	}
}

func TestListLimitsRequireExplicitUnlimited(t *testing.T) {
	t.Parallel()

	zero, err := crud.NewListPreparer(
		crud.WithApplicationDefaults(
			crud.WithFilterLimits(crud.FilterLimits{}),
			crud.WithOrderLimits(crud.OrderLimits{}),
		),
	)
	if err != nil {
		t.Fatalf("NewListPreparer with zero limits: %v", err)
	}
	settings := zero.SettingsFor("example.servora.dev/User")
	if got, limited := settings.FilterLimits().MaxBytes(); !limited || got != 8*1024 {
		t.Fatalf("zero filter MaxBytes = (%d, %v), want default (8192, true)", got, limited)
	}
	if got, limited := settings.OrderLimits().MaxTerms(); !limited || got != 8 {
		t.Fatalf("zero order MaxTerms = (%d, %v), want default (8, true)", got, limited)
	}

	unlimited, err := crud.NewListPreparer(
		crud.WithApplicationDefaults(
			crud.WithFilterLimits(crud.UnlimitedFilterLimits()),
			crud.WithOrderLimits(crud.UnlimitedOrderLimits()),
			crud.WithoutMaxPageSize(),
		),
	)
	if err != nil {
		t.Fatalf("NewListPreparer with unlimited limits: %v", err)
	}
	settings = unlimited.SettingsFor("example.servora.dev/User")
	if _, limited := settings.FilterLimits().MaxBytes(); limited {
		t.Fatal("UnlimitedFilterLimits still limits bytes")
	}
	if _, limited := settings.OrderLimits().MaxTerms(); limited {
		t.Fatal("UnlimitedOrderLimits still limits terms")
	}
	if _, limited := settings.MaxPageSize(); limited {
		t.Fatal("WithoutMaxPageSize still limits page size")
	}
}

func TestPrepareListAppliesPageSizeSettings(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer(
		crud.WithResourceOverrides(
			"example.servora.dev/User",
			crud.WithDefaultPageSize(25),
			crud.WithMaxPageSize(100),
		),
	)
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	resource := listResource("example.servora.dev/User")

	query, err := preparer.PrepareList(resource, crud.ListInput{PageSize: 0})
	if err != nil {
		t.Fatalf("PrepareList default: %v", err)
	}
	if got, want := query.PageSize(), int32(25); got != want {
		t.Fatalf("default PageSize = %d, want %d", got, want)
	}

	query, err = preparer.PrepareList(resource, crud.ListInput{PageSize: 250})
	if err != nil {
		t.Fatalf("PrepareList capped: %v", err)
	}
	if got, want := query.PageSize(), int32(100); got != want {
		t.Fatalf("capped PageSize = %d, want %d", got, want)
	}

	_, err = preparer.PrepareList(resource, crud.ListInput{PageSize: -1})
	if !crudpb.IsCrudErrorReasonInvalidFieldValue(err) {
		t.Fatalf("negative page size error = %v, want INVALID_FIELD_VALUE", err)
	}
}

func TestPrepareListPreservesCollection(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	query, err := preparer.PrepareList(
		listResource("example.servora.dev/User"),
		crud.ListInput{Collection: "tenants/acme"},
	)
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	if got, want := query.Collection(), "tenants/acme"; got != want {
		t.Fatalf("Collection() = %q, want %q", got, want)
	}
}

func TestNewListPreparerRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options []crud.ListPreparerOption
	}{
		{
			name: "non-positive default page size",
			options: []crud.ListPreparerOption{
				crud.WithApplicationDefaults(crud.WithDefaultPageSize(0)),
			},
		},
		{
			name: "default exceeds maximum",
			options: []crud.ListPreparerOption{
				crud.WithApplicationDefaults(crud.WithDefaultPageSize(100), crud.WithMaxPageSize(50)),
			},
		},
		{
			name: "empty resource type",
			options: []crud.ListPreparerOption{
				crud.WithResourceOverrides("", crud.WithDefaultPageSize(10)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := crud.NewListPreparer(test.options...); err == nil {
				t.Fatal("NewListPreparer error = nil")
			}
		})
	}
}

func TestPrepareListRejectsTypedNilResource(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer()
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	var plan *crud.ResourcePlan[*examplev1.User]
	if _, err := preparer.PrepareList(plan, crud.ListInput{}); err == nil {
		t.Fatal("PrepareList accepted a typed-nil resource")
	}
}

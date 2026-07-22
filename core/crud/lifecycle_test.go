package crud_test

import (
	"slices"
	"testing"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPrepareCreateValidatesRequiredAndClearsSystemFields(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	if _, err := plan.PrepareCreate(&examplev1.User{}); !crudpb.IsCrudErrorReasonInvalidFieldValue(err) {
		t.Fatalf("PrepareCreate missing required error = %v, want INVALID_FIELD_VALUE", err)
	}
	prepared, err := plan.PrepareCreate(&examplev1.User{
		Name:              "tenants/acme/users/u-1",
		Email:             proto.String("alice@example.com"),
		TemporaryPassword: proto.String("secret"),
		CreateTime:        timestamppb.Now(),
	})
	if err != nil {
		t.Fatalf("PrepareCreate: %v", err)
	}
	resource := prepared.Resource()
	if resource.GetName() != "" || resource.GetCreateTime() != nil {
		t.Fatalf("system fields survived PrepareCreate: %v", resource)
	}
	if got, want := resource.GetTemporaryPassword(), "secret"; got != want {
		t.Fatalf("temporary_password = %q, want %q", got, want)
	}
}

func TestPrepareUpdateBuildsMutableLeafMaskAndClear(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	prepared, err := plan.PrepareUpdate(
		&examplev1.User{Name: "tenants/acme/users/u-1"},
		&fieldmaskpb.FieldMask{Paths: []string{"nickname", "create_time"}},
		crud.UpdateOptions{AllowMissing: true, Etag: "v1"},
	)
	if err != nil {
		t.Fatalf("PrepareUpdate: %v", err)
	}
	if got, want := prepared.WriteMask().GetPaths(), []string{"nickname"}; !slices.Equal(got, want) {
		t.Fatalf("WriteMask paths = %v, want %v", got, want)
	}
	if prepared.Resource().Nickname != nil {
		t.Fatal("absent optional nickname did not remain a Clear intent")
	}
	if !prepared.Options().AllowMissing || prepared.Options().Etag != "v1" {
		t.Fatalf("Options() = %+v", prepared.Options())
	}
}

func TestPreparedUpdateValidatesImmutableDirectIntent(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	prepared, err := plan.PrepareUpdate(
		&examplev1.User{TenantPlan: proto.String("enterprise")},
		&fieldmaskpb.FieldMask{Paths: []string{"tenant_plan"}},
		crud.UpdateOptions{},
	)
	if err != nil {
		t.Fatalf("PrepareUpdate: %v", err)
	}
	if got, want := len(prepared.ImmutableComparisons()), 1; got != want {
		t.Fatalf("immutable comparisons = %d, want %d", got, want)
	}
	if err := prepared.ValidateImmutable(&examplev1.User{TenantPlan: proto.String("enterprise")}); err != nil {
		t.Fatalf("ValidateImmutable same value: %v", err)
	}
	if err := prepared.ValidateImmutable(&examplev1.User{TenantPlan: proto.String("basic")}); !crudpb.IsCrudErrorReasonInvalidFieldValue(err) {
		t.Fatalf("ValidateImmutable changed value error = %v, want INVALID_FIELD_VALUE", err)
	}
	if slices.Contains(prepared.WriteMask().GetPaths(), "tenant_plan") {
		t.Fatal("immutable field leaked into mutable WriteMask")
	}
}

func TestPrepareUpdateImplicitMaskIgnoresDefaultPlainScalar(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	prepared, err := plan.PrepareUpdate(&examplev1.User{Email: proto.String("alice@example.com")}, nil, crud.UpdateOptions{})
	if err != nil {
		t.Fatalf("PrepareUpdate: %v", err)
	}
	if got, want := prepared.WriteMask().GetPaths(), []string{"email"}; !slices.Equal(got, want) {
		t.Fatalf("WriteMask paths = %v, want %v", got, want)
	}
}

func TestToResponseValidatesNameClonesAndClearsInputOnly(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	input := &examplev1.User{
		Name:              "tenants/acme/users/u-1",
		TemporaryPassword: proto.String("secret"),
	}
	output, err := plan.ToResponse(input)
	if err != nil {
		t.Fatalf("ToResponse: %v", err)
	}
	if output.GetTemporaryPassword() != "" {
		t.Fatal("ToResponse retained INPUT_ONLY temporary_password")
	}
	if input.GetTemporaryPassword() != "secret" {
		t.Fatal("ToResponse mutated input")
	}
	output.Name = "changed"
	if input.GetName() != "tenants/acme/users/u-1" {
		t.Fatal("ToResponse returned aliased input")
	}
}

func TestToResponseRejectsInvalidCanonicalName(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	for _, name := range []string{"", "users/u-1", "tenants/acme/users"} {
		if _, err := plan.ToResponse(&examplev1.User{Name: name}); err == nil {
			t.Fatalf("ToResponse accepted invalid name %q", name)
		}
	}
}

func TestToResponsesRejectsNilElement(t *testing.T) {
	t.Parallel()

	plan := userResourcePlan()
	if _, err := plan.ToResponses([]*examplev1.User{{Name: "tenants/acme/users/u-1"}, nil}); err == nil {
		t.Fatal("ToResponses accepted nil element")
	}
}

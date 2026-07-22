package crud_test

import (
	"reflect"
	"testing"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
	annotations "google.golang.org/genproto/googleapis/api/annotations"
)

func TestMustBuildResourcePlan(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)

	if got, want := plan.Descriptor().FullName(), descriptor.FullName(); got != want {
		t.Fatalf("Descriptor().FullName() = %q, want %q", got, want)
	}
	if got, want := plan.ResourceType(), "example.servora.dev/User"; got != want {
		t.Fatalf("ResourceType() = %q, want %q", got, want)
	}
	if got, want := plan.Patterns(), []string{"tenants/{tenant}/users/{user}"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Patterns() = %v, want %v", got, want)
	}
	if got, want := string(plan.Identifier().Name()), "name"; got != want {
		t.Fatalf("Identifier().Name() = %q, want %q", got, want)
	}

	assertBehavior(t, plan, "email", annotations.FieldBehavior_REQUIRED)
	assertBehavior(t, plan, "tenant_plan", annotations.FieldBehavior_IMMUTABLE)
	assertBehavior(t, plan, "temporary_password", annotations.FieldBehavior_INPUT_ONLY)
	assertBehavior(t, plan, "create_time", annotations.FieldBehavior_OUTPUT_ONLY)

	wantWritable := []string{"display_name", "email", "nickname", "temporary_password"}
	if got := plan.WritablePaths(); !reflect.DeepEqual(got, wantWritable) {
		t.Fatalf("WritablePaths() = %v, want %v", got, wantWritable)
	}
}

func TestResourcePlanAccessorsReturnCopies(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)

	patterns := plan.Patterns()
	patterns[0] = "mutated/{value}"
	if got := plan.Patterns()[0]; got == patterns[0] {
		t.Fatal("Patterns returned mutable plan storage")
	}

	paths := plan.WritablePaths()
	paths[0] = "mutated"
	if got := plan.WritablePaths()[0]; got == paths[0] {
		t.Fatal("WritablePaths returned mutable plan storage")
	}
}

func assertBehavior(
	t *testing.T,
	plan *crud.ResourcePlan[*examplev1.User],
	path string,
	behavior annotations.FieldBehavior,
) {
	t.Helper()

	field, ok := plan.Field(path)
	if !ok {
		t.Fatalf("Field(%q) not found", path)
	}
	if !field.HasBehavior(behavior) {
		t.Fatalf("Field(%q) does not have behavior %s", path, behavior)
	}
}

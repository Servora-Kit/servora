package crud_test

import (
	"reflect"
	"testing"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
)

func TestResourceNameMatcherParseFormatRoundTrip(t *testing.T) {
	t.Parallel()

	matcher, err := crud.NewResourceNameMatcher(
		"users/{user}",
		"tenants/{tenant}/users/{user}",
	)
	if err != nil {
		t.Fatalf("NewResourceNameMatcher: %v", err)
	}

	parsed, err := matcher.Parse("tenants/acme/users/u-1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := parsed.Pattern(), "tenants/{tenant}/users/{user}"; got != want {
		t.Fatalf("Pattern = %q, want %q", got, want)
	}
	wantVariables := map[string]string{"tenant": "acme", "user": "u-1"}
	if got := parsed.Variables(); !reflect.DeepEqual(got, wantVariables) {
		t.Fatalf("Variables = %v, want %v", got, wantVariables)
	}
	formatted, err := matcher.Format(parsed.Pattern(), parsed.Variables())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got, want := formatted, "tenants/acme/users/u-1"; got != want {
		t.Fatalf("Format = %q, want %q", got, want)
	}
}

func TestResourceNameMatcherTreatsDashAsOrdinaryValue(t *testing.T) {
	t.Parallel()

	matcher, err := crud.NewResourceNameMatcher("tenants/{tenant}/users/{user}")
	if err != nil {
		t.Fatalf("NewResourceNameMatcher: %v", err)
	}
	parsed, err := matcher.Parse("tenants/-/users/u-1")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, ok := parsed.Variable("tenant"); !ok || got != "-" {
		t.Fatalf("tenant = (%q, %v), want (\"-\", true)", got, ok)
	}
}

func TestResourceNameMatcherRejectsAmbiguousSkeletons(t *testing.T) {
	t.Parallel()

	_, err := crud.NewResourceNameMatcher(
		"tenants/{tenant}/users/{user}",
		"tenants/{organization}/users/{member}",
	)
	if err == nil {
		t.Fatal("NewResourceNameMatcher error = nil")
	}
}

func TestResourceNameMatcherRejectsInvalidFormatValues(t *testing.T) {
	t.Parallel()

	matcher, err := crud.NewResourceNameMatcher("tenants/{tenant}/users/{user}")
	if err != nil {
		t.Fatalf("NewResourceNameMatcher: %v", err)
	}

	tests := []struct {
		name      string
		variables map[string]string
	}{
		{"missing variable", map[string]string{"tenant": "acme"}},
		{"empty segment", map[string]string{"tenant": "acme", "user": ""}},
		{"slash in segment", map[string]string{"tenant": "acme", "user": "a/b"}},
		{"unknown variable", map[string]string{"tenant": "acme", "user": "u-1", "extra": "x"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := matcher.Format("tenants/{tenant}/users/{user}", test.variables); err == nil {
				t.Fatal("Format error = nil")
			}
		})
	}
}

func TestResourcePlanParsesCanonicalName(t *testing.T) {
	t.Parallel()

	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	parsed, err := plan.ParseName("tenants/acme/users/u-1")
	if err != nil {
		t.Fatalf("ParseName: %v", err)
	}
	if got, ok := parsed.Variable("user"); !ok || got != "u-1" {
		t.Fatalf("user = (%q, %v), want (\"u-1\", true)", got, ok)
	}
}

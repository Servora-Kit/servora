package crud

import (
	"fmt"
	"strings"
	"testing"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestClearHelperDefaultsSameNameNullableField(t *testing.T) {
	t.Parallel()

	helper, err := NewClearHelper[*fakeClearMutation]()
	if err != nil {
		t.Fatalf("NewClearHelper: %v", err)
	}
	mutation := newFakeClearMutation("nickname")
	resource := &examplev1.User{}
	if err := helper.Apply(resource, &fieldmaskpb.FieldMask{Paths: []string{"nickname"}}, mutation); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := strings.Join(mutation.cleared, ","), "nickname"; got != want {
		t.Fatalf("cleared = %q, want %q", got, want)
	}

	mutation = newFakeClearMutation("nickname")
	resource.Nickname = proto.String("present")
	if err := helper.Apply(resource, &fieldmaskpb.FieldMask{Paths: []string{"nickname"}}, mutation); err != nil {
		t.Fatalf("Apply present: %v", err)
	}
	if len(mutation.cleared) != 0 {
		t.Fatalf("present field was cleared: %v", mutation.cleared)
	}
}

func TestClearHelperSupportsRenameAndClearToValue(t *testing.T) {
	t.Parallel()

	helper, err := NewClearHelper(
		RenameClear[*fakeClearMutation](examplev1.UserFields.Email, "contact_email"),
		ClearToValue(examplev1.UserFields.DisplayName, func(mutation *fakeClearMutation) error {
			mutation.displayName = ""
			mutation.displayNameSet = true
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("NewClearHelper: %v", err)
	}
	mutation := newFakeClearMutation("contact_email")
	resource := &examplev1.User{}
	mask := &fieldmaskpb.FieldMask{Paths: []string{"email", "display_name"}}
	if err := helper.Apply(resource, mask, mutation); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := strings.Join(mutation.cleared, ","), "contact_email"; got != want {
		t.Fatalf("cleared = %q, want %q", got, want)
	}
	if !mutation.displayNameSet || mutation.displayName != "" {
		t.Fatalf("display_name clear-to-value was not applied: %#v", mutation)
	}
}

func TestClearHelperFailsBeforeSaveWhenClearIsUnsupported(t *testing.T) {
	t.Parallel()

	helper, err := NewClearHelper[*fakeClearMutation]()
	if err != nil {
		t.Fatalf("NewClearHelper: %v", err)
	}
	mutation := newFakeClearMutation("nickname")
	err = helper.Apply(&examplev1.User{}, &fieldmaskpb.FieldMask{Paths: []string{"email"}}, mutation)
	if err == nil || !strings.Contains(err.Error(), "same-name Ent field") {
		t.Fatalf("Apply error = %v, want unsupported same-name clear", err)
	}
	if mutation.saved {
		t.Fatal("mutation was saved after Clear failure")
	}
}

func TestClearHelperRejectsUnnormalizedOrPresenceLessMask(t *testing.T) {
	t.Parallel()

	helper, err := NewClearHelper[*fakeClearMutation]()
	if err != nil {
		t.Fatalf("NewClearHelper: %v", err)
	}
	mutation := newFakeClearMutation("nickname")
	for _, test := range []struct {
		name string
		mask *fieldmaskpb.FieldMask
		want string
	}{
		{name: "wildcard", mask: &fieldmaskpb.FieldMask{Paths: []string{"*"}}, want: "still contains wildcard"},
		{name: "no presence", mask: &fieldmaskpb.FieldMask{Paths: []string{"etag"}}, want: "has no Proto presence"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := helper.Apply(&examplev1.User{}, test.mask, mutation)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Apply error = %v, want containing %q", err, test.want)
			}
		})
	}
}

type fakeClearMutation struct {
	clearable      map[string]struct{}
	cleared        []string
	displayName    string
	displayNameSet bool
	saved          bool
}

func newFakeClearMutation(clearable ...string) *fakeClearMutation {
	fields := make(map[string]struct{}, len(clearable))
	for _, field := range clearable {
		fields[field] = struct{}{}
	}
	return &fakeClearMutation{clearable: fields}
}

func (mutation *fakeClearMutation) ClearField(field string) error {
	if _, ok := mutation.clearable[field]; !ok {
		return fmt.Errorf("unknown nullable field %s", field)
	}
	mutation.cleared = append(mutation.cleared, field)
	return nil
}

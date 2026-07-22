package mapper_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	crudmapper "github.com/Servora-Kit/servora/core/crud/mapper"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type userPO struct {
	TenantID          string
	ID                string
	DisplayName       string
	Email             string
	TemporaryPassword string
	CreatedAt         time.Time
}

type testFieldPath struct {
	path       string
	descriptor protoreflect.FieldDescriptor
}

func (path testFieldPath) String() string                           { return path.path }
func (path testFieldPath) Descriptor() protoreflect.FieldDescriptor { return path.descriptor }

func TestResourceMapperProjectsFieldsOptionsAndCanonicalName(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)
	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, userPO](
		crudmapper.WithFieldMapping("CreatedAt", userFieldPath("create_time")),
		crudmapper.WithPostToDTOHook(func(po *userPO, dto *examplev1.User) error {
			dto.Nickname = &po.DisplayName
			return nil
		}),
		crudmapper.WithResourceName(func(po *userPO) (string, error) {
			return fmt.Sprintf("tenants/%s/users/%s", po.TenantID, po.ID), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	po := &userPO{
		TenantID: "acme", ID: "u-1", DisplayName: "Alice", Email: "alice@example.com",
		TemporaryPassword: "secret", CreatedAt: createdAt,
	}
	dto, err := mapper.TryToDTO(po)
	if err != nil {
		t.Fatalf("TryToDTO: %v", err)
	}
	if dto.GetName() != "tenants/acme/users/u-1" || dto.GetDisplayName() != "Alice" || dto.GetEmail() != "alice@example.com" {
		t.Fatalf("mapped DTO = %v", dto)
	}
	if dto.GetCreateTime().AsTime() != createdAt {
		t.Fatalf("create_time = %v, want %v", dto.GetCreateTime(), createdAt)
	}
	if dto.GetNickname() != "Alice" {
		t.Fatalf("hook nickname = %q, want Alice", dto.GetNickname())
	}
	po.DisplayName = "changed"
	if dto.GetNickname() != "Alice" {
		t.Fatalf("hook output aliases PO: nickname = %q", dto.GetNickname())
	}
	*dto.Nickname = "response"
	if po.DisplayName != "changed" {
		t.Fatalf("DTO mutation changed PO display name to %q", po.DisplayName)
	}
}

func TestResourceMapperAppliesMultipleExplicitFieldMappings(t *testing.T) {
	t.Parallel()

	type mappedPO struct {
		Primary   string
		Secondary string
	}
	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, mappedPO](
		crudmapper.WithFieldMapping("Primary", userFieldPath("display_name")),
		crudmapper.WithFieldMapping("Secondary", userFieldPath("email")),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	dto := mapper.ToDTO(&mappedPO{Primary: "first", Secondary: "second@example.com"})
	if dto.GetDisplayName() != "first" || dto.GetEmail() != "second@example.com" {
		t.Fatalf("mapped DTO = %v", dto)
	}
}

func TestResourceMapperUsesExplicitConverter(t *testing.T) {
	t.Parallel()

	type convertedPO struct {
		TenantID    string
		ID          string
		DisplayName int
	}
	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, convertedPO](
		crudmapper.WithConverters(crudmapper.TypeConverter{
			SrcType: int(0),
			DstType: (*string)(nil),
			Fn: func(source any) (any, error) {
				value := fmt.Sprintf("user-%d", source.(int))
				return &value, nil
			},
		}),
		crudmapper.WithResourceName(func(po *convertedPO) (string, error) {
			return fmt.Sprintf("tenants/%s/users/%s", po.TenantID, po.ID), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	if got := mapper.ToDTO(&convertedPO{TenantID: "acme", ID: "u-1", DisplayName: 7}).GetDisplayName(); got != "user-7" {
		t.Fatalf("DisplayName = %q, want user-7", got)
	}
}

func TestResourceMapperTryRejectsMalformedConvertersWithoutPanicking(t *testing.T) {
	t.Parallel()

	type convertedPO struct{ DisplayName int }
	tests := []struct {
		name string
		fn   func(any) (any, error)
	}{
		{name: "wrong result type", fn: func(any) (any, error) { return 7, nil }},
		{name: "panic", fn: func(any) (any, error) { panic("boom") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mapper, err := crudmapper.NewResourceMapper[*examplev1.User, convertedPO](
				crudmapper.WithConverters(crudmapper.TypeConverter{SrcType: int(0), DstType: (*string)(nil), Fn: test.fn}),
			)
			if err != nil {
				t.Fatalf("NewResourceMapper: %v", err)
			}
			if _, err := mapper.TryToDTO(&convertedPO{DisplayName: 1}); err == nil {
				t.Fatal("TryToDTO accepted malformed converter result")
			}
		})
	}
}

func TestResourceMapperRejectsLossyImplicitSameNameConversion(t *testing.T) {
	t.Parallel()

	type displayName string
	type incompatiblePO struct{ DisplayName displayName }
	if _, err := crudmapper.NewResourceMapper[*examplev1.User, incompatiblePO](); err == nil {
		t.Fatal("NewResourceMapper accepted implicit named-string conversion")
	}
}

func TestResourceMapperRejectsInvalidBuiltinTimestamp(t *testing.T) {
	t.Parallel()

	type timestampPO struct{ CreateTime time.Time }
	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, timestampPO]()
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	if _, err := mapper.TryToDTO(&timestampPO{CreateTime: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}); err == nil {
		t.Fatal("TryToDTO accepted out-of-range timestamp")
	}
}

func TestResourceMapperRejectsInvalidFieldMappingAtConstruction(t *testing.T) {
	t.Parallel()

	for _, option := range []crudmapper.Option{
		crudmapper.WithFieldMapping("Missing", userFieldPath("create_time")),
		crudmapper.WithFieldMapping("CreatedAt", testFieldPath{path: "profile.bio", descriptor: userFieldPath("display_name").Descriptor()}),
	} {
		if _, err := crudmapper.NewResourceMapper[*examplev1.User, userPO](option); err == nil {
			t.Fatal("NewResourceMapper accepted invalid field mapping")
		}
	}
}

func TestResourceMapperNilAndBatchContracts(t *testing.T) {
	t.Parallel()

	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, userPO]()
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	if dto, err := mapper.TryToDTO(nil); err != nil || dto != nil {
		t.Fatalf("TryToDTO(nil) = (%v, %v), want (nil, nil)", dto, err)
	}
	if mapper.ToDTO(nil) != nil {
		t.Fatal("ToDTO(nil) is not nil")
	}
	if dtos, err := mapper.TryToDTOs(nil); err != nil || dtos != nil {
		t.Fatalf("TryToDTOs(nil) = (%v, %v), want (nil, nil)", dtos, err)
	}
	first := &userPO{DisplayName: "first"}
	second := &userPO{DisplayName: "second"}
	dtos, err := mapper.TryToDTOs([]*userPO{first, second})
	if err != nil {
		t.Fatalf("TryToDTOs: %v", err)
	}
	if got, want := []string{dtos[0].GetDisplayName(), dtos[1].GetDisplayName()}, []string{"first", "second"}; !slices.Equal(got, want) {
		t.Fatalf("batch order = %v, want %v", got, want)
	}
	if _, err := mapper.TryToDTOs([]*userPO{first, nil}); err == nil {
		t.Fatal("TryToDTOs accepted nil element")
	}
	assertPanics(t, func() { mapper.ToDTOs([]*userPO{first, nil}) })
}

func TestResourceMapperTryReturnsHookErrorAndToPanics(t *testing.T) {
	t.Parallel()

	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, userPO](
		crudmapper.WithPostToDTOHook(func(*userPO, *examplev1.User) error {
			return errors.New("hook failed")
		}),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	if _, err := mapper.TryToDTO(&userPO{}); err == nil {
		t.Fatal("TryToDTO did not return hook error")
	}
	assertPanics(t, func() { mapper.ToDTO(&userPO{}) })
}

func TestResourceMapperFormatterErrorPanicsInMustAPI(t *testing.T) {
	t.Parallel()

	mapper, err := crudmapper.NewResourceMapper[*examplev1.User, userPO](
		crudmapper.WithResourceName(func(*userPO) (string, error) { return "", errors.New("format failed") }),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}
	assertPanics(t, func() { mapper.ToDTO(&userPO{}) })
}

func userFieldPath(name protoreflect.Name) testFieldPath {
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	return testFieldPath{path: string(name), descriptor: descriptor.Fields().ByName(name)}
}

func assertPanics(t *testing.T, call func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("call did not panic")
		}
	}()
	call()
}

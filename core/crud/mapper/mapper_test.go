package mapper_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	crudmapper "github.com/Servora-Kit/servora/core/crud/mapper"
	"github.com/Servora-Kit/servora/core/crud/mapper/testdata/nestedpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestResourceMapperProjectsNestedMessagesOnDemandAndPreventsTopLevelCollision(t *testing.T) {
	t.Parallel()

	type accountPO struct {
		ID           string
		Email        *string
		Name         *string
		Nickname     *string
		Bio          string
		Rank         int
		Tags         []string
		VerifiedTime *time.Time
		City         *string
		PostalCode   string
	}

	mapper, err := crudmapper.NewResourceMapper[*nestedpb.Account, accountPO](
		crudmapper.WithFieldMapping("Name", nestedpb.AccountFields.ProfileName),
		crudmapper.WithFieldMapping("Nickname", nestedpb.AccountFields.ProfileNickname),
		crudmapper.WithFieldMapping("Bio", nestedpb.AccountFields.ProfileBio),
		crudmapper.WithFieldMapping("Rank", nestedpb.AccountFields.ProfileRank),
		crudmapper.WithFieldMapping("Tags", nestedpb.AccountFields.ProfileTags),
		crudmapper.WithFieldMapping("VerifiedTime", nestedpb.AccountFields.ProfileVerifiedTime),
		crudmapper.WithFieldMapping("City", nestedpb.AccountFields.ProfileAddressCity),
		crudmapper.WithFieldMapping("PostalCode", nestedpb.AccountFields.ProfileAddressPostalCode),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}

	t.Run("all empty leaves nested messages nil", func(t *testing.T) {
		t.Parallel()
		email := "empty@example.com"
		dto := mapper.ToDTO(&accountPO{
			ID:    "acc-empty",
			Email: &email,
			Tags:  []string{},
		})
		if dto.GetEmail() != email {
			t.Fatalf("Email = %q, want %q", dto.GetEmail(), email)
		}
		if dto.Profile != nil {
			t.Fatalf("Profile = %+v, want nil when all nested sources are empty", dto.Profile)
		}
	})

	t.Run("partial single-level leaves deeper sub-message nil and blocks top-level Name collision", func(t *testing.T) {
		t.Parallel()
		name := "Alice"
		dto := mapper.ToDTO(&accountPO{
			ID:   "acc-1",
			Name: &name,
			Bio:  "builder",
		})
		if dto.GetName() != "" {
			t.Fatalf("top-level IDENTIFIER Name = %q, want empty without WithResourceName", dto.GetName())
		}
		if dto.Profile == nil {
			t.Fatal("Profile is nil, want initialized")
		}
		if dto.GetProfile().GetName() != "Alice" || dto.GetProfile().GetBio() != "builder" {
			t.Fatalf("Profile = %+v", dto.GetProfile())
		}
		if dto.GetProfile().Nickname != nil {
			t.Fatalf("Profile.Nickname = %v, want nil", dto.GetProfile().Nickname)
		}
		if dto.GetProfile().Address != nil {
			t.Fatalf("Profile.Address = %+v, want nil when address fields are empty", dto.GetProfile().Address)
		}

		name = "MutatedPO"
		if dto.GetProfile().GetName() != "Alice" {
			t.Fatalf("Profile.Name aliased PO pointer: got %q", dto.GetProfile().GetName())
		}
		*dto.Profile.Name = "MutatedDTO"
		if name != "MutatedPO" {
			t.Fatalf("mutating DTO changed PO Name to %q", name)
		}
	})

	t.Run("non-nil empty string pointer allocates sub-message and preserves presence", func(t *testing.T) {
		t.Parallel()
		empty := ""
		dto := mapper.ToDTO(&accountPO{Nickname: &empty})
		if dto.Profile == nil || dto.Profile.Nickname == nil || *dto.Profile.Nickname != "" {
			t.Fatalf("Profile.Nickname = %v, want non-nil empty string", dto.GetProfile().Nickname)
		}
	})

	t.Run("multi-level nested fields initialize ancestor chain and deep-copy slices", func(t *testing.T) {
		t.Parallel()
		verified := time.Date(2026, time.November, 13, 8, 30, 0, 0, time.UTC)
		city := "Shanghai"
		tags := []string{"admin", "ops"}
		dto := mapper.ToDTO(&accountPO{
			Rank:         3,
			Tags:         tags,
			VerifiedTime: &verified,
			City:         &city,
			PostalCode:   "200000",
		})
		if dto.GetProfile().GetRank() != 3 {
			t.Fatalf("Profile.Rank = %d, want 3", dto.GetProfile().GetRank())
		}
		if !slices.Equal(dto.GetProfile().GetTags(), []string{"admin", "ops"}) {
			t.Fatalf("Profile.Tags = %v", dto.GetProfile().GetTags())
		}
		if !dto.GetProfile().GetVerifiedTime().AsTime().Equal(verified) {
			t.Fatalf("Profile.VerifiedTime = %v, want %v", dto.GetProfile().GetVerifiedTime(), verified)
		}
		if dto.GetProfile().GetAddress().GetCity() != "Shanghai" || dto.GetProfile().GetAddress().GetPostalCode() != "200000" {
			t.Fatalf("Profile.Address = %+v", dto.GetProfile().GetAddress())
		}
		tags[0] = "mutated"
		if dto.GetProfile().GetTags()[0] != "admin" {
			t.Fatalf("Profile.Tags aliased PO slice: %v", dto.GetProfile().GetTags())
		}
	})
}

func TestResourceMapperNestedConvertersAndErrorPropagation(t *testing.T) {
	t.Parallel()

	type customPO struct {
		Code int
	}
	mapper, err := crudmapper.NewResourceMapper[*nestedpb.Account, customPO](
		crudmapper.WithConverters(crudmapper.TypeConverter{
			SrcType: int(0),
			DstType: (*string)(nil),
			Fn: func(source any) (any, error) {
				code := source.(int)
				if code < 0 {
					return nil, errors.New("negative code")
				}
				if code == 999 {
					return (*string)(nil), nil
				}
				formatted := fmt.Sprintf("BIO-%d", code)
				return &formatted, nil
			},
		}),
		crudmapper.WithFieldMapping("Code", nestedpb.AccountFields.ProfileBio),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper: %v", err)
	}

	if dto := mapper.ToDTO(&customPO{Code: 42}); dto.GetProfile().GetBio() != "BIO-42" {
		t.Fatalf("Profile.Bio = %q, want BIO-42", dto.GetProfile().GetBio())
	}
	if dto := mapper.ToDTO(&customPO{Code: 999}); dto.Profile != nil {
		t.Fatalf("Profile = %+v, want nil when converter returns typed nil pointer", dto.Profile)
	}
	if _, err := mapper.TryToDTO(&customPO{Code: -1}); err == nil {
		t.Fatal("TryToDTO accepted failing nested converter")
	}
	assertPanics(t, func() { mapper.ToDTO(&customPO{Code: -1}) })
}

func TestResourceMapperRejectsInvalidNestedMappingsAtConstruction(t *testing.T) {
	t.Parallel()

	type nestedTestPO struct {
		A    string
		B    string
		When *timestamppb.Timestamp
		Sub  *nestedpb.Profile
	}

	accountDesc := nestedpb.AccountCRUDDescriptor()
	tagLabelDesc := accountDesc.Fields().ByName("items").Message().Fields().ByName("label")
	mapValueLabelDesc := accountDesc.Fields().ByName("labels").MapValue().Message().Fields().ByName("label")
	tsSecondsDesc := accountDesc.Fields().ByName("create_time").Message().Fields().ByName("seconds")

	cases := []struct {
		name    string
		options []crudmapper.Option
	}{
		{
			name: "repeated message traversal",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "items.label", descriptor: tagLabelDesc}),
			},
		},
		{
			name: "map field traversal",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "labels.label", descriptor: mapValueLabelDesc}),
			},
		},
		{
			name: "top-level WKT traversal",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "create_time.seconds", descriptor: tsSecondsDesc}),
			},
		},
		{
			name: "nested WKT traversal",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "profile.verified_time.seconds", descriptor: tsSecondsDesc}),
			},
		},
		{
			name: "unknown intermediate segment",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "unknown.bio", descriptor: nestedpb.AccountFields.ProfileBio.Descriptor()}),
			},
		},
		{
			name: "unknown leaf segment",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "profile.unknown", descriptor: nestedpb.AccountFields.ProfileBio.Descriptor()}),
			},
		},
		{
			name: "empty path segment",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "profile..bio", descriptor: nestedpb.AccountFields.ProfileBio.Descriptor()}),
			},
		},
		{
			name: "mismatched leaf descriptor",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", testFieldPath{path: "profile.bio", descriptor: nestedpb.AccountFields.Email.Descriptor()}),
			},
		},
		{
			name: "parent then child overlap",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("Sub", nestedpb.AccountFields.Profile),
				crudmapper.WithFieldMapping("A", nestedpb.AccountFields.ProfileBio),
			},
		},
		{
			name: "child then parent overlap",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", nestedpb.AccountFields.ProfileBio),
				crudmapper.WithFieldMapping("Sub", nestedpb.AccountFields.Profile),
			},
		},
		{
			name: "duplicate nested target",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", nestedpb.AccountFields.ProfileBio),
				crudmapper.WithFieldMapping("B", nestedpb.AccountFields.ProfileBio),
			},
		},
		{
			name: "incompatible nested leaf type",
			options: []crudmapper.Option{
				crudmapper.WithFieldMapping("A", nestedpb.AccountFields.ProfileRank),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := crudmapper.NewResourceMapper[*nestedpb.Account, nestedTestPO](tc.options...); err == nil {
				t.Fatalf("NewResourceMapper accepted invalid nested mapping (%s)", tc.name)
			}
		})
	}

	t.Run("implicit same-name parent conflicts with nested child", func(t *testing.T) {
		t.Parallel()
		type conflictingPO struct {
			Profile *nestedpb.Profile
			Bio     string
		}
		if _, err := crudmapper.NewResourceMapper[*nestedpb.Account, conflictingPO](
			crudmapper.WithFieldMapping("Bio", nestedpb.AccountFields.ProfileBio),
		); err == nil {
			t.Fatal("NewResourceMapper accepted implicit parent copy alongside nested child mapping")
		}
	})
}

func TestResourceMapperWithFlattenSingleMultiLevelAndExplicitOverride(t *testing.T) {
	t.Parallel()

	type flattenedPO struct {
		ID           string
		Email        string
		Name         *string
		Nickname     *string
		AvatarCustom *string
		VerifiedTime *time.Time
		City         *string
		PostalCode   string
	}

	mapper, err := crudmapper.NewResourceMapper[*nestedpb.Account, flattenedPO](
		crudmapper.WithFlatten(nestedpb.AccountFields.Profile, nestedpb.AccountFields.ProfileAddress),
		crudmapper.WithFieldMapping("AvatarCustom", nestedpb.AccountFields.ProfileBio),
		crudmapper.WithResourceName(func(po *flattenedPO) (string, error) {
			return "accounts/" + po.ID, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewResourceMapper WithFlatten: %v", err)
	}

	// All nested fields absent -> Profile stays nil while top-level fields and Name formatter work.
	emptyDTO := mapper.ToDTO(&flattenedPO{ID: "a-1", Email: "a@example.com"})
	if emptyDTO.GetName() != "accounts/a-1" || emptyDTO.GetEmail() != "a@example.com" || emptyDTO.Profile != nil {
		t.Fatalf("empty flattened DTO = %+v, want nil Profile", emptyDTO)
	}

	// Multi-level only (City set, direct Profile scalars nil) -> initializes Profile and Address on demand.
	city := "Shanghai"
	deepDTO := mapper.ToDTO(&flattenedPO{ID: "a-2", Email: "b@example.com", City: &city})
	if deepDTO.GetProfile().GetAddress().GetCity() != "Shanghai" || deepDTO.GetProfile().Name != nil {
		t.Fatalf("deep flattened DTO = %+v", deepDTO)
	}

	// Full projection including Name (without colliding with IDENTIFIER), explicit override, and WKT timestamp.
	personName := "Alice"
	bio := "custom-bio"
	verifiedAt := time.Date(2026, time.November, 13, 8, 0, 0, 0, time.UTC)
	fullDTO := mapper.ToDTO(&flattenedPO{
		ID:           "a-3",
		Email:        "c@example.com",
		Name:         &personName,
		AvatarCustom: &bio,
		VerifiedTime: &verifiedAt,
		City:         &city,
		PostalCode:   "200000",
	})
	if fullDTO.GetName() != "accounts/a-3" {
		t.Fatalf("IDENTIFIER Name = %q, want accounts/a-3", fullDTO.GetName())
	}
	if fullDTO.GetProfile().GetName() != "Alice" || fullDTO.GetProfile().GetBio() != "custom-bio" {
		t.Fatalf("Profile = %+v", fullDTO.GetProfile())
	}
	if !fullDTO.GetProfile().GetVerifiedTime().AsTime().Equal(verifiedAt) {
		t.Fatalf("VerifiedTime = %v, want %v", fullDTO.GetProfile().GetVerifiedTime(), verifiedAt)
	}
	if fullDTO.GetProfile().GetAddress().GetPostalCode() != "200000" {
		t.Fatalf("PostalCode = %q, want 200000", fullDTO.GetProfile().GetAddress().GetPostalCode())
	}

	// Invalid WithFlatten configurations rejected at construction.
	type ambiguousIdentifierPO struct {
		Name *string
	}
	if _, err := crudmapper.NewResourceMapper[*nestedpb.Account, ambiguousIdentifierPO](
		crudmapper.WithFlatten(nestedpb.AccountFields.Profile),
	); err == nil {
		t.Fatal("expected error when flattened PO.Name collides with IDENTIFIER without WithResourceName")
	}
	for _, badOpt := range []crudmapper.Option{
		crudmapper.WithFlatten(),
		crudmapper.WithFlatten(nestedpb.AccountFields.Email),
		crudmapper.WithFlatten(nestedpb.AccountFields.Items),
		crudmapper.WithFlatten(nestedpb.AccountFields.CreateTime),
		crudmapper.WithFlatten(nestedpb.AccountFields.Profile, nestedpb.AccountFields.Profile),
	} {
		if _, err := crudmapper.NewResourceMapper[*nestedpb.Account, flattenedPO](
			badOpt,
			crudmapper.WithResourceName(func(po *flattenedPO) (string, error) { return "accounts/" + po.ID, nil }),
		); err == nil {
			t.Fatal("expected invalid WithFlatten option to fail")
		}
	}
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

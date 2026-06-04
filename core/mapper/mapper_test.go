package mapper

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type domainUser struct {
	ID        int64
	Name      string
	Email     string
	Password  string `protobuf:"bytes,4,opt,name=password,proto3" json:"password,omitempty"`
	Phone     *string
	Role      string
	Profile   *profileDTO `protobuf:"bytes,7,opt,name=profile,proto3" json:"profile,omitempty"`
	Tags      []string    `protobuf:"bytes,8,rep,name=tags,proto3" json:"tags,omitempty"`
	CreatedAt *timestamppb.Timestamp
	UserID    string
	Score     int32
}

type profileDTO struct {
	DisplayName string
}

type entLikeUser struct {
	ID        int64
	Name      string
	Email     string
	Password  string
	Phone     *string
	Role      string
	Profile   *profileDTO
	Tags      []string
	CreatedAt time.Time
	UserUUID  uuid.UUID
	Score     int
}

func TestCopierMapper_ToDTO(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()

	src := &entLikeUser{ID: 7, Name: "alice", Email: "alice@example.com", Password: "hashed", Role: "admin"}
	dst := m.ToDTO(src)

	require.NotNil(t, dst)
	require.Equal(t, src.ID, dst.ID)
	require.Equal(t, src.Name, dst.Name)
}

func TestCopierMapper_TryToDTO_Nil(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()

	dst, err := m.TryToDTO(nil)

	require.NoError(t, err)
	require.Nil(t, dst)
}

func TestCopierMapper_ToDTOList(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()

	entities := []*entLikeUser{{ID: 1, Name: "a"}, nil, {ID: 2, Name: "b"}}
	dtos := m.ToDTOList(entities)

	require.Len(t, dtos, 2)
	require.Equal(t, int64(1), dtos[0].ID)
	require.Equal(t, int64(2), dtos[1].ID)
}

func TestCopierMapper_DefaultConverters(t *testing.T) {
	type dto struct {
		ID        string
		CreatedAt *timestamppb.Timestamp
		Score     int32
		Phone     *string
	}
	type entity struct {
		ID        uuid.UUID
		CreatedAt time.Time
		Score     int
		Phone     string
	}

	now := time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC)
	id := uuid.New()
	m := NewCopierMapper[dto, entity]()

	got := m.ToDTO(&entity{ID: id, CreatedAt: now, Score: 42, Phone: "13800000000"})

	require.Equal(t, id.String(), got.ID)
	require.Equal(t, now, got.CreatedAt.AsTime())
	require.Equal(t, int32(42), got.Score)
	require.NotNil(t, got.Phone)
	require.Equal(t, "13800000000", *got.Phone)
}

func TestCopierMapper_ApplyRename(t *testing.T) {
	type dto struct {
		Name string
	}
	type entity struct {
		UserName string
	}

	m := NewCopierMapper[dto, entity]()
	err := Apply(&Config{FieldMapping: map[string]string{"UserName": "Name"}}, m)
	require.NoError(t, err)

	got := m.ToDTO(&entity{UserName: "alice"})

	require.Equal(t, "alice", got.Name)
}

func TestCopierMapper_IgnoreRead(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	err := Apply(&Config{IgnoreRead: []string{"password", "profile", "tags"}}, m)
	require.NoError(t, err)

	got := m.ToDTO(&entLikeUser{
		ID:       1,
		Name:     "alice",
		Password: "hashed",
		Profile:  &profileDTO{DisplayName: "Alice"},
		Tags:     []string{"admin"},
	})

	require.Equal(t, int64(1), got.ID)
	require.Empty(t, got.Password)
	require.Nil(t, got.Profile)
	require.Nil(t, got.Tags)
}

func TestCopierMapper_PostToDTOHook(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.WithPostToDTOHook(func(entity *entLikeUser, dto *domainUser) error {
		dto.Role = entity.Role + "_from_hook"
		return nil
	})

	got := m.ToDTO(&entLikeUser{ID: 1, Role: "admin"})

	require.Equal(t, "admin_from_hook", got.Role)
}

func TestCopierMapper_TryToDTO_ReturnsHookError(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.WithPostToDTOHook(func(_ *entLikeUser, _ *domainUser) error {
		return fmt.Errorf("hook failed")
	})

	_, err := m.TryToDTO(&entLikeUser{ID: 1})

	require.Error(t, err)
	require.Contains(t, err.Error(), "hook failed")
}

func TestCopierMapper_AppendBusinessEnumConverter(t *testing.T) {
	type dtoStatus int32
	type entityStatus string
	type dto struct {
		Status dtoStatus
	}
	type entity struct {
		Status entityStatus
	}
	const (
		dtoStatusUnspecified dtoStatus = 0
		dtoStatusActive      dtoStatus = 1
	)

	converter := NewEnumConverter[dtoStatus, entityStatus](
		map[int32]string{0: "UNSPECIFIED", 1: "ACTIVE"},
		map[string]int32{"UNSPECIFIED": 0, "ACTIVE": 1},
	)
	m := NewCopierMapper[dto, entity]()
	m.AppendConverters(converter.NewConverterPair())

	got := m.ToDTO(&entity{Status: "ACTIVE"})

	require.Equal(t, dtoStatusActive, got.Status)
	require.NotEqual(t, dtoStatusUnspecified, got.Status)
}

func TestApplyNilConfig(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	require.NoError(t, Apply(nil, m))
}

func TestApplyNilMapper(t *testing.T) {
	err := Apply[domainUser, entLikeUser](&Config{}, nil)
	require.Error(t, err)
}

type typeA struct {
	ID   int
	Name string
}

type typeB struct {
	Ident string
	Label string
}

func newTestMapper() *Mapper[typeA, typeB] {
	return NewMapper(
		func(a *typeA) *typeB {
			return &typeB{Ident: string(rune(a.ID + '0')), Label: a.Name}
		},
		func(b *typeB) *typeA {
			return &typeA{ID: int(b.Ident[0] - '0'), Name: b.Label}
		},
	)
}

func TestMapper_Map(t *testing.T) {
	m := newTestMapper()
	b := m.Map(&typeA{ID: 1, Name: "hello"})
	require.NotNil(t, b)
	require.Equal(t, "1", b.Ident)
	require.Equal(t, "hello", b.Label)
}

func TestMapper_Reverse(t *testing.T) {
	m := newTestMapper()
	a := m.Reverse(&typeB{Ident: "3", Label: "world"})
	require.NotNil(t, a)
	require.Equal(t, 3, a.ID)
	require.Equal(t, "world", a.Name)
}

func TestMapper_MapNil(t *testing.T) {
	m := newTestMapper()
	require.Nil(t, m.Map(nil))
	require.Nil(t, m.Reverse(nil))
}

func TestMapper_MapSlice(t *testing.T) {
	m := newTestMapper()
	as := []*typeA{{ID: 1, Name: "a"}, nil, {ID: 2, Name: "b"}}
	bs := m.MapSlice(as)
	require.Len(t, bs, 2)
	require.Equal(t, "1", bs[0].Ident)
	require.Equal(t, "2", bs[1].Ident)
}

func TestMapper_ReverseSlice(t *testing.T) {
	m := newTestMapper()
	bs := []*typeB{{Ident: "5", Label: "x"}, nil}
	as := m.ReverseSlice(bs)
	require.Len(t, as, 1)
	require.Equal(t, 5, as[0].ID)
}

func TestMapper_EmptySlice(t *testing.T) {
	m := newTestMapper()
	require.Nil(t, m.MapSlice(nil))
	require.Nil(t, m.MapSlice([]*typeA{}))
	require.Nil(t, m.ReverseSlice(nil))
	require.Nil(t, m.ReverseSlice([]*typeB{}))
}

func TestForwardMapper_ReverseReturnsNil(t *testing.T) {
	m := NewForwardMapper(func(a *typeA) *typeB {
		return &typeB{Ident: "x", Label: a.Name}
	})
	b := m.Map(&typeA{Name: "test"})
	require.NotNil(t, b)

	require.Nil(t, m.Reverse(&typeB{Ident: "x"}))
	result := m.ReverseSlice([]*typeB{{Ident: "x"}})
	require.Empty(t, result)
}

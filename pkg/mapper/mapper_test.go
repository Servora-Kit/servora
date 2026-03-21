package mapper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------- test types ----------

type domainUser struct {
	ID       int64
	Name     string
	Email    string
	Password string
	Phone    *string
	Role     string
}

type entLikeUser struct {
	ID       int64
	Name     string
	Email    string
	Password string
	Phone    *string
	Role     string
}

// ---------- CopierMapper (unified) ----------

func TestCopierMapper_ToProto(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.AppendConverters(AllBuiltinConverters())

	src := &entLikeUser{ID: 7, Name: "alice", Email: "alice@example.com", Password: "hashed", Role: "admin"}
	dst, err := m.ToProto(src)
	require.NoError(t, err)
	require.NotNil(t, dst)
	require.Equal(t, src.ID, dst.ID)
	require.Equal(t, src.Name, dst.Name)
}

func TestCopierMapper_ToEntity(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.AppendConverters(AllBuiltinConverters())

	src := &domainUser{ID: 3, Name: "bob", Email: "bob@example.com", Role: "user"}
	dst, err := m.ToEntity(src)
	require.NoError(t, err)
	require.NotNil(t, dst)
	require.Equal(t, src.ID, dst.ID)
}

func TestCopierMapper_ErrorReturn(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	dst, err := m.ToProto(nil)
	require.NoError(t, err)
	require.Nil(t, dst)
}

func TestCopierMapper_MustToProto(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.AppendConverters(AllBuiltinConverters())

	src := &entLikeUser{ID: 1, Name: "test"}
	dst := m.MustToProto(src)
	require.NotNil(t, dst)
	require.Equal(t, int64(1), dst.ID)
}

func TestCopierMapper_ListConversions(t *testing.T) {
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.AppendConverters(AllBuiltinConverters())

	entities := []*entLikeUser{{ID: 1, Name: "a"}, nil, {ID: 2, Name: "b"}}
	protos, err := m.ToProtoList(entities)
	require.NoError(t, err)
	require.Len(t, protos, 2)
}

func TestCopierMapper_RoundTrip(t *testing.T) {
	phone := "13800000000"
	m := NewCopierMapper[domainUser, entLikeUser]()
	m.AppendConverters(AllBuiltinConverters())

	original := &domainUser{
		ID:       7,
		Name:     "alice",
		Email:    "alice@example.com",
		Password: "hashed-password",
		Phone:    &phone,
		Role:     "admin",
	}

	entity, err := m.ToEntity(original)
	require.NoError(t, err)
	require.NotNil(t, entity)
	require.Equal(t, original.ID, entity.ID)
	require.Equal(t, original.Name, entity.Name)

	back, err := m.ToProto(entity)
	require.NoError(t, err)
	require.NotNil(t, back)
	require.Equal(t, original, back)
}

func TestCopierMapper_ToEntityListWithNilItems(t *testing.T) {
	phone := "13900000000"
	entities := []*entLikeUser{
		{ID: 1, Name: "u1", Email: "u1@example.com", Phone: &phone, Role: "user"},
		nil,
		{ID: 2, Name: "u2", Email: "u2@example.com", Role: "admin"},
	}

	m := NewCopierMapper[domainUser, entLikeUser]()
	m.AppendConverters(AllBuiltinConverters())
	protos, err := m.ToProtoList(entities)
	require.NoError(t, err)
	require.Len(t, protos, 2)
	require.Equal(t, int64(1), protos[0].ID)
	require.Equal(t, int64(2), protos[1].ID)
}

// ---------- Functional Mapper ----------

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

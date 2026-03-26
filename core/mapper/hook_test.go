package mapper

import (
	"testing"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/require"
)

func TestHookRegistry_RegisterAndGet(t *testing.T) {
	r := NewHookRegistry()
	r.Register("user_profile", copier.TypeConverter{
		SrcType: "", DstType: "",
		Fn: func(src any) (any, error) { return src, nil },
	})

	cs, ok := r.Get("user_profile")
	require.True(t, ok)
	require.Len(t, cs, 1)
}

func TestHookRegistry_GetMissing(t *testing.T) {
	r := NewHookRegistry()
	_, ok := r.Get("nonexistent")
	require.False(t, ok)
}

func TestHookRegistry_MustGet_Panics(t *testing.T) {
	r := NewHookRegistry()
	require.Panics(t, func() { r.MustGet("nonexistent") })
}

func TestHookRegistry_CheckMissing(t *testing.T) {
	r := NewHookRegistry()
	r.Register("a", copier.TypeConverter{})

	err := r.CheckRequired("a", "b", "c")
	require.Error(t, err)
	require.Contains(t, err.Error(), "b")
	require.Contains(t, err.Error(), "c")
}

func TestHookRegistry_CheckAllPresent(t *testing.T) {
	r := NewHookRegistry()
	r.Register("a", copier.TypeConverter{})
	r.Register("b", copier.TypeConverter{})

	err := r.CheckRequired("a", "b")
	require.NoError(t, err)
}

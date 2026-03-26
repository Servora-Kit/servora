package mapper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPresetRegistry_GetBuiltin(t *testing.T) {
	r := NewPresetRegistry()
	r.RegisterDefaults()

	cs, ok := r.Get("proto_time")
	require.True(t, ok)
	require.NotEmpty(t, cs)
}

func TestPresetRegistry_GetUnknown(t *testing.T) {
	r := NewPresetRegistry()
	_, ok := r.Get("nonexistent")
	require.False(t, ok)
}

func TestPresetRegistry_Collect(t *testing.T) {
	r := NewPresetRegistry()
	r.RegisterDefaults()

	cs, err := r.Collect("proto_time", "pointer")
	require.NoError(t, err)
	require.NotEmpty(t, cs)
}

func TestPresetRegistry_CollectUnknown(t *testing.T) {
	r := NewPresetRegistry()
	_, err := r.Collect("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent")
}

func TestPresetRegistry_CommonProtoEntity(t *testing.T) {
	r := NewPresetRegistry()
	r.RegisterDefaults()

	cs, ok := r.Get("common_proto_entity")
	require.True(t, ok)
	require.NotEmpty(t, cs)
}

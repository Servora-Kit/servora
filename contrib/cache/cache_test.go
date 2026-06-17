package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryStoreTTLAndDelete(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	require.NoError(t, store.Set(ctx, "a", []byte("value"), time.Hour))
	got, err := store.Get(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []byte("value"), got)

	require.NoError(t, store.Delete(ctx, "a"))
	got, err = store.Get(ctx, "a")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSupportNoCache(t *testing.T) {
	_, err := NoCache().Get(context.Background(), "a")

	require.ErrorIs(t, err, ErrNoCache)
}

func TestKeyspaceInvalidationChangesKeys(t *testing.T) {
	ks := NewKeyspace("jobs")
	before := ks.Key("list", map[string]string{"b": "2", "a": "1"})

	ks.Invalidate(context.Background())
	after := ks.Key("list", map[string]string{"a": "1", "b": "2"})

	require.NotEqual(t, before, after)
	require.Equal(t, StableKey([]string{"b", "a"}), StableKey([]string{"a", "b"}))
}

func TestSupportSingleflightAndHooks(t *testing.T) {
	ctx := context.Background()
	var misses atomic.Int64
	var sets atomic.Int64
	support := NewSupport(NewMemoryStore(), Options{
		TTL:          time.Hour,
		Singleflight: true,
		Hooks: Hooks{
			OnMiss: func(context.Context, string) { misses.Add(1) },
			OnSet:  func(context.Context, string) { sets.Add(1) },
		},
	})

	var loads atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := support.GetOrLoad(ctx, "job:1", 0, func(context.Context) ([]byte, error) {
				loads.Add(1)
				time.Sleep(10 * time.Millisecond)
				return []byte("cached"), nil
			})
			require.NoError(t, err)
			require.Equal(t, []byte("cached"), got)
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), loads.Load())
	require.GreaterOrEqual(t, misses.Load(), int64(1))
	require.Equal(t, int64(1), sets.Load())
}

func TestSupportCacheEmptyUsesNegativeTTL(t *testing.T) {
	ctx := context.Background()
	support := NewSupport(NewMemoryStore(), Options{
		CacheEmpty:  true,
		NegativeTTL: time.Hour,
	})

	got, err := support.GetOrLoad(ctx, "empty", time.Hour, func(context.Context) ([]byte, error) {
		return nil, nil
	})
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = support.Get(ctx, "empty")
	require.NoError(t, err)
	require.Nil(t, got)
}

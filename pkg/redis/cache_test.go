package redis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type testUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func marshalUser(u testUser) (string, error) {
	b, err := json.Marshal(u)
	return string(b), err
}

func unmarshalUser(s string) (testUser, error) {
	var u testUser
	err := json.Unmarshal([]byte(s), &u)
	return u, err
}

func TestGetOrSet_CacheHit(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "test:cache:hit"
	expected := testUser{ID: 1, Name: "Alice"}

	data, _ := marshalUser(expected)
	c.Set(ctx, key, data, time.Minute)

	loaderCalled := false
	result, err := GetOrSet(ctx, c, key, time.Minute,
		func(_ context.Context) (testUser, error) {
			loaderCalled = true
			return testUser{}, nil
		},
		marshalUser, unmarshalUser,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaderCalled {
		t.Fatal("loader should not be called on cache hit")
	}
	if result.ID != expected.ID || result.Name != expected.Name {
		t.Fatalf("expected %+v, got %+v", expected, result)
	}
}

func TestGetOrSet_CacheMiss(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "test:cache:miss"
	expected := testUser{ID: 2, Name: "Bob"}

	result, err := GetOrSet(ctx, c, key, time.Minute,
		func(_ context.Context) (testUser, error) { return expected, nil },
		marshalUser, unmarshalUser,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID || result.Name != expected.Name {
		t.Fatalf("expected %+v, got %+v", expected, result)
	}

	cached, cacheErr := c.Get(ctx, key)
	if cacheErr != nil {
		t.Fatalf("value should be cached: %v", cacheErr)
	}
	var cachedUser testUser
	json.Unmarshal([]byte(cached), &cachedUser)
	if cachedUser.ID != expected.ID {
		t.Fatalf("cached value mismatch: %+v", cachedUser)
	}
}

func TestGetOrSet_LoaderError(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "test:cache:loader-err"
	loaderErr := errors.New("db down")

	_, err := GetOrSet(ctx, c, key, time.Minute,
		func(_ context.Context) (testUser, error) { return testUser{}, loaderErr },
		marshalUser, unmarshalUser,
	)
	if !errors.Is(err, loaderErr) {
		t.Fatalf("expected loader error, got %v", err)
	}

	if c.Has(ctx, key) {
		t.Fatal("key should not be cached on loader error")
	}
}

func TestGetOrSetJSON_CacheHit(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "test:cache:json-hit"
	expected := testUser{ID: 3, Name: "Charlie"}

	data, _ := json.Marshal(expected)
	c.Set(ctx, key, string(data), time.Minute)

	result, err := GetOrSetJSON(ctx, c, key, time.Minute,
		func(_ context.Context) (testUser, error) {
			t.Fatal("loader should not be called")
			return testUser{}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID || result.Name != expected.Name {
		t.Fatalf("expected %+v, got %+v", expected, result)
	}
}

func TestGetOrSetJSON_CacheMiss(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "test:cache:json-miss"
	expected := testUser{ID: 4, Name: "Diana"}

	result, err := GetOrSetJSON(ctx, c, key, time.Minute,
		func(_ context.Context) (testUser, error) { return expected, nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID {
		t.Fatalf("expected %+v, got %+v", expected, result)
	}
}

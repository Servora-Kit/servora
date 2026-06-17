package cache

import (
	"context"
	"sync"
	"time"
)

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

type MemoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]memoryItem)}
}

func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[key]
	if !ok {
		return nil, nil
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		delete(s.items, key)
		return nil, nil
	}
	return append([]byte(nil), item.value...), nil
}

func (s *MemoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := memoryItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = item
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range keys {
		delete(s.items, key)
	}
	return nil
}

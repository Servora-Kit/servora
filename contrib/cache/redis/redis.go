package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client redis.UniversalClient
}

func NewStore(client redis.UniversalClient) *Store {
	return &Store{client: client}
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return value, err
}

func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *Store) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return s.client.Del(ctx, keys...).Err()
}

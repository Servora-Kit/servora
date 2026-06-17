package cache

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/singleflight"
)

var ErrNoCache = errors.New("cache: disabled")

type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type Codec[T any] interface {
	Marshal(T) ([]byte, error)
	Unmarshal([]byte) (T, error)
}

type Hooks struct {
	OnHit    func(context.Context, string)
	OnMiss   func(context.Context, string)
	OnError  func(context.Context, string, error)
	OnSet    func(context.Context, string)
	OnDelete func(context.Context, []string)
}

type Options struct {
	TTL              time.Duration
	CacheEmpty       bool
	Singleflight     bool
	NegativeTTL      time.Duration
	DefaultKeyPrefix string
	Hooks            Hooks
}

type Support struct {
	store Store
	opts  Options
	group singleflight.Group
}

func NewSupport(store Store, opts Options) *Support {
	if store == nil {
		return NoCache()
	}
	return &Support{store: store, opts: opts}
}

func NoCache() *Support {
	return &Support{}
}

func (s *Support) Enabled() bool {
	return s != nil && s.store != nil
}

func (s *Support) Get(ctx context.Context, key string) ([]byte, error) {
	if !s.Enabled() {
		return nil, ErrNoCache
	}
	value, err := s.store.Get(ctx, key)
	if err != nil {
		if s.opts.Hooks.OnError != nil {
			s.opts.Hooks.OnError(ctx, key, err)
		}
		return nil, err
	}
	if len(value) == 0 {
		if s.opts.Hooks.OnMiss != nil {
			s.opts.Hooks.OnMiss(ctx, key)
		}
		return nil, nil
	}
	if s.opts.Hooks.OnHit != nil {
		s.opts.Hooks.OnHit(ctx, key)
	}
	return value, nil
}

func (s *Support) GetOrLoad(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) ([]byte, error)) ([]byte, error) {
	if !s.Enabled() {
		return nil, ErrNoCache
	}
	value, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if value != nil {
		return value, nil
	}
	if loader == nil {
		return nil, nil
	}

	load := func() ([]byte, error) {
		loaded, err := loader(ctx)
		if err != nil {
			if s.opts.Hooks.OnError != nil {
				s.opts.Hooks.OnError(ctx, key, err)
			}
			return nil, err
		}
		setTTL := ttl
		if len(loaded) == 0 && s.opts.NegativeTTL > 0 {
			setTTL = s.opts.NegativeTTL
		}
		if err := s.Set(ctx, key, loaded, setTTL); err != nil {
			return nil, err
		}
		return loaded, nil
	}
	if !s.opts.Singleflight {
		return load()
	}
	shared, err, _ := s.group.Do(key, func() (any, error) {
		return load()
	})
	if err != nil {
		return nil, err
	}
	if shared == nil {
		return nil, nil
	}
	return shared.([]byte), nil
}

func (s *Support) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if !s.Enabled() {
		return ErrNoCache
	}
	if len(value) == 0 && !s.opts.CacheEmpty {
		return nil
	}
	if ttl == 0 {
		ttl = s.opts.TTL
	}
	if err := s.store.Set(ctx, key, value, ttl); err != nil {
		if s.opts.Hooks.OnError != nil {
			s.opts.Hooks.OnError(ctx, key, err)
		}
		return err
	}
	if s.opts.Hooks.OnSet != nil {
		s.opts.Hooks.OnSet(ctx, key)
	}
	return nil
}

func (s *Support) Delete(ctx context.Context, keys ...string) error {
	if !s.Enabled() {
		return ErrNoCache
	}
	if len(keys) == 0 {
		return nil
	}
	if err := s.store.Delete(ctx, keys...); err != nil {
		if s.opts.Hooks.OnError != nil {
			s.opts.Hooks.OnError(ctx, keys[0], err)
		}
		return err
	}
	if s.opts.Hooks.OnDelete != nil {
		s.opts.Hooks.OnDelete(ctx, keys)
	}
	return nil
}

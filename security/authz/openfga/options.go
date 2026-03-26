package openfga

import (
	"time"

	"github.com/Servora-Kit/servora/infra/redis"
)

// Option configures the OpenFGA Authorizer.
type Option func(*authorizerConfig)

type authorizerConfig struct {
	redis    *redis.Client
	cacheTTL time.Duration
}

// WithRedisCache enables Redis caching of authorization check results.
// Results are stored for the given TTL. Pass nil redis client to disable caching.
func WithRedisCache(rdb *redis.Client, ttl time.Duration) Option {
	return func(c *authorizerConfig) {
		c.redis = rdb
		c.cacheTTL = ttl
	}
}

// Package openfga provides an OpenFGA-based Authorizer implementation for pkg/authz.
// Use NewAuthorizer to create an instance and pass it to authz.Server().
package openfga

import (
	"context"

	"github.com/Servora-Kit/servora/security/authz"
	pkgfga "github.com/Servora-Kit/servora/infra/openfga"
)

// Ensure *Authorizer implements authz.Authorizer at compile time.
var _ authz.Authorizer = (*Authorizer)(nil)

// Authorizer is an OpenFGA-based authorization engine.
// It optionally caches results in Redis via the WithRedisCache option.
type Authorizer struct {
	client *pkgfga.Client
	cfg    *authorizerConfig
}

// NewAuthorizer creates an OpenFGA-backed Authorizer.
// The fgaClient must not be nil; pass WithRedisCache to enable result caching.
func NewAuthorizer(fgaClient *pkgfga.Client, opts ...Option) authz.Authorizer {
	cfg := &authorizerConfig{
		cacheTTL: pkgfga.DefaultCheckCacheTTL,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Authorizer{client: fgaClient, cfg: cfg}
}

// IsAuthorized checks whether subject has the given relation on objectType:objectID.
// If a Redis cache is configured, results are cached for the configured TTL.
// The CacheHit field in DecisionDetail reflects whether the result came from cache.
func (a *Authorizer) IsAuthorized(ctx context.Context, subject, relation, objectType, objectID string) (bool, error) {
	allowed, _, err := a.client.CachedCheck(ctx, a.cfg.redis, a.cfg.cacheTTL,
		subject, relation, objectType, objectID)
	return allowed, err
}

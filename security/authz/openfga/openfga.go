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

// Check uses CachedCheck (which falls back to plain Check when redis is nil).
// Cache-hit signals stay inside this package — they are not exposed via DecisionDetail.
func (a *Authorizer) Check(ctx context.Context, subject, relation, objectType, objectID string) (bool, error) {
	allowed, _, err := a.client.CachedCheck(ctx, a.cfg.redis, a.cfg.cacheTTL,
		subject, relation, objectType, objectID)
	return allowed, err
}

// BatchCheck delegates to *openfga.Client.BatchCheck.
// Cache is intentionally NOT consulted for batch checks — N Redis lookups would
// negate the batching win. Callers needing cached batch behavior should issue
// N Check calls instead.
func (a *Authorizer) BatchCheck(ctx context.Context, reqs []authz.CheckRequest) ([]authz.CheckResult, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	items := make([]pkgfga.BatchCheckItem, len(reqs))
	for i, r := range reqs {
		items[i] = pkgfga.BatchCheckItem{
			User:     r.Subject,
			Relation: r.Relation,
			Object:   r.ObjectType + ":" + r.ObjectID,
		}
	}

	results, err := a.client.BatchCheck(ctx, items)
	if err != nil {
		return nil, err
	}

	out := make([]authz.CheckResult, len(results))
	for i, r := range results {
		out[i] = authz.CheckResult{Allowed: r.Allowed, Err: r.Err}
	}
	return out, nil
}

// ListAllowed delegates to *openfga.Client.CachedListObjects (cache opt-in).
func (a *Authorizer) ListAllowed(ctx context.Context, subject, relation, objectType string) ([]string, error) {
	return a.client.CachedListObjects(ctx, a.cfg.redis, pkgfga.DefaultListCacheTTL,
		subject, relation, objectType)
}

// Package openfga provides an OpenFGA-based Authorizer implementation for
// security/authz. The concrete Authorizer type satisfies three interfaces:
//
//   - authz.Authorizer   (single Check)
//   - batch.BatchAuthorizer (BatchCheck in one round-trip)
//   - lister.Lister       (ListAllowed — enumerate accessible resources)
//
// Use NewAuthorizer to create an instance and pass it to authz.Server().
package openfga

import (
	"context"
	"time"

	"github.com/Servora-Kit/servora/infra/redis"
	"github.com/Servora-Kit/servora/security/authz"
	"github.com/Servora-Kit/servora/security/authz/batch"
	"github.com/Servora-Kit/servora/security/authz/lister"
)

// Compile-time interface assertions.
var (
	_ authz.Authorizer     = (*Authorizer)(nil)
	_ batch.BatchAuthorizer = (*Authorizer)(nil)
	_ lister.Lister         = (*Authorizer)(nil)
)

// Authorizer is an OpenFGA-based authorization engine.
// It optionally caches results in Redis via the WithRedisCache option.
type Authorizer struct {
	client *Client
	redis  *redis.Client
	ttl    time.Duration
}

// AuthorizerOption configures the OpenFGA Authorizer.
type AuthorizerOption func(*Authorizer)

// WithRedisCache enables Redis caching of authorization check results.
// Results are stored for the given TTL. Pass nil redis client to disable caching.
func WithRedisCache(rdb *redis.Client, ttl time.Duration) AuthorizerOption {
	return func(a *Authorizer) {
		a.redis = rdb
		a.ttl = ttl
	}
}

// NewAuthorizer creates an OpenFGA-backed Authorizer.
// The fgaClient must not be nil; pass WithRedisCache to enable result caching.
//
// The returned value satisfies authz.Authorizer. Callers that need BatchCheck
// or ListAllowed can type-assert to batch.BatchAuthorizer or lister.Lister.
func NewAuthorizer(fgaClient *Client, opts ...AuthorizerOption) *Authorizer {
	a := &Authorizer{
		client: fgaClient,
		ttl:    DefaultCheckCacheTTL,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Check implements authz.Authorizer. It maps CheckRequest fields to OpenFGA
// tuple format: Subject → User, Action → Relation, ResourceType:ResourceID → Object.
func (a *Authorizer) Check(ctx context.Context, req authz.CheckRequest) (bool, error) {
	allowed, _, err := a.client.CachedCheck(
		ctx, a.redis, a.ttl,
		req.Subject, req.Action, req.ResourceType, req.ResourceID,
	)
	return allowed, err
}

// BatchCheck implements batch.BatchAuthorizer. It runs N checks in one OpenFGA
// call. Cache is intentionally NOT consulted — N Redis lookups would negate the
// batching win. Output order matches input order.
func (a *Authorizer) BatchCheck(ctx context.Context, reqs []authz.CheckRequest) ([]bool, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	items := make([]BatchCheckItem, len(reqs))
	for i, r := range reqs {
		items[i] = BatchCheckItem{
			User:     r.Subject,
			Relation: r.Action,
			Object:   r.ResourceType + ":" + r.ResourceID,
		}
	}

	results, err := a.client.BatchCheck(ctx, items)
	if err != nil {
		return nil, err
	}

	out := make([]bool, len(results))
	for i, r := range results {
		if r.Err != nil {
			return nil, r.Err
		}
		out[i] = r.Allowed
	}
	return out, nil
}

// ListAllowed implements lister.Lister. It returns IDs of resources (of
// resourceType) the subject has the given action on.
func (a *Authorizer) ListAllowed(ctx context.Context, subject, action, resourceType string) ([]string, error) {
	return a.client.CachedListObjects(ctx, a.redis, DefaultListCacheTTL,
		subject, action, resourceType)
}

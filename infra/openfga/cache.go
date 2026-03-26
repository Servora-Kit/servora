package openfga

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Servora-Kit/servora/infra/redis"
)

const (
	DefaultCheckCacheTTL = 60 * time.Second
	DefaultListCacheTTL  = 10 * time.Minute
)

// CachedCheck is like Check but caches results in Redis. If the Redis client
// is nil the call degrades to a plain Check. The second return value indicates
// whether the result was served from cache.
func (c *Client) CachedCheck(ctx context.Context, rdb *redis.Client, ttl time.Duration,
	user, relation, objectType, objectID string) (allowed bool, cacheHit bool, err error) {

	if rdb == nil {
		allowed, err = c.Check(ctx, user, relation, objectType, objectID)
		return allowed, false, err
	}

	key := checkCacheKey(user, relation, objectType, objectID)

	cached, getErr := rdb.Get(ctx, key)
	if getErr == nil {
		return cached == "1", true, nil
	}

	allowed, err = c.Check(ctx, user, relation, objectType, objectID)
	if err != nil {
		return false, false, err
	}

	_ = rdb.Set(ctx, key, boolStr(allowed), ttl)
	return allowed, false, nil
}

// CachedListObjects is like ListObjects but caches the full ID list in Redis.
// Subsequent calls within the TTL window return the cached result, avoiding
// repeated OpenFGA round-trips.  Returns all IDs; the caller is responsible
// for pagination.
func (c *Client) CachedListObjects(ctx context.Context, rdb *redis.Client, ttl time.Duration,
	user, relation, objectType string) ([]string, error) {

	if rdb == nil {
		return c.ListObjects(ctx, user, relation, objectType)
	}

	key := listCacheKey(user, relation, objectType)

	members, err := rdb.SMembers(ctx, key)
	if err == nil && len(members) > 0 {
		return members, nil
	}

	ids, err := c.ListObjects(ctx, user, relation, objectType)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return ids, nil
	}

	vals := make([]any, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	_ = rdb.SAdd(ctx, key, vals...)
	_ = rdb.Expire(ctx, key, ttl)

	return ids, nil
}

// InvalidateCheck removes a cached Check result.
func InvalidateCheck(ctx context.Context, rdb *redis.Client, user, relation, objectType, objectID string) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(ctx, checkCacheKey(user, relation, objectType, objectID))
}

// InvalidateListObjects removes a cached ListObjects result.
func InvalidateListObjects(ctx context.Context, rdb *redis.Client, user, relation, objectType string) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(ctx, listCacheKey(user, relation, objectType))
}

// InvalidateForTuples invalidates all cached Check and ListObjects entries
// that could be affected by the given tuples. This should be called after
// WriteTuples or DeleteTuples to keep the cache consistent.
//
// For each tuple it invalidates:
//   - The exact Check cache entry (user + relation + object)
//   - The ListObjects cache for the user on the object's type with the tuple's relation
//   - Additional computed relations as configured via WithComputedRelations
func (c *Client) InvalidateForTuples(ctx context.Context, rdb *redis.Client, tuples []Tuple) {
	if rdb == nil || len(tuples) == 0 {
		return
	}

	var keys []string
	seen := make(map[string]struct{})

	for _, t := range tuples {
		user, objectType, objectID := parseTupleComponents(t)
		if user == "" || objectType == "" {
			continue
		}

		if objectID != "" {
			k := checkCacheKey(user, t.Relation, objectType, objectID)
			if _, ok := seen[k]; !ok {
				keys = append(keys, k)
				seen[k] = struct{}{}
			}
		}

		for _, rel := range c.affectedRelations(t.Relation, objectType) {
			k := listCacheKey(user, rel, objectType)
			if _, ok := seen[k]; !ok {
				keys = append(keys, k)
				seen[k] = struct{}{}
			}
		}
	}

	for _, k := range keys {
		_ = rdb.Del(ctx, k)
	}
}

// parseTupleComponents extracts the user principal, objectType, and objectID from a Tuple.
// Tuple.User is e.g. "user:abc", "service:gateway", or "organization:org-1#member".
// Tuple.Object is e.g. "organization:xyz".
func parseTupleComponents(t Tuple) (user, objectType, objectID string) {
	if i := strings.IndexByte(t.User, ':'); i >= 0 {
		user = t.User[i+1:]
	}
	if i := strings.IndexByte(t.Object, ':'); i >= 0 {
		objectType = t.Object[:i]
		objectID = t.Object[i+1:]
	}
	return
}

// affectedRelations returns the tuple's own relation plus computed relations
// configured via WithComputedRelations for the given object type.
func (c *Client) affectedRelations(relation, objectType string) []string {
	rels := []string{relation}
	if c.computedRelations != nil {
		if computed, ok := c.computedRelations[objectType]; ok {
			rels = append(rels, computed...)
		}
	}
	return rels
}

func checkCacheKey(userID, relation, objectType, objectID string) string {
	return fmt.Sprintf("authz:check:%s:%s:%s:%s", userID, relation, objectType, objectID)
}

func listCacheKey(userID, relation, objectType string) string {
	return fmt.Sprintf("authz:list:%s:%s:%s", userID, relation, objectType)
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

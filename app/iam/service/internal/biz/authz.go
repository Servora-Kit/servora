package biz

import (
	"context"
	"time"
)

const DefaultListCacheTTL = 10 * time.Minute

// Tuple represents a relationship tuple for authorization (e.g. "user:X is owner of organization:Y").
type Tuple struct {
	User     string
	Relation string
	Object   string
}

// AuthZRepo defines the authorization capability interface.
// Implementations live in the data layer (e.g. backed by OpenFGA + Redis cache).
type AuthZRepo interface {
	WriteTuples(ctx context.Context, tuples ...Tuple) error
	DeleteTuples(ctx context.Context, tuples ...Tuple) error
	Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error)
	ListObjects(ctx context.Context, userID, relation, objectType string) ([]string, error)
	CachedListObjects(ctx context.Context, ttl time.Duration, userID, relation, objectType string) ([]string, error)
	InvalidateListObjects(ctx context.Context, userID, relation, objectType string)
}

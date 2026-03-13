package data

import (
	"context"
	"time"

	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/Servora-Kit/servora/pkg/redis"
)

type authzRepo struct {
	fga *openfga.Client
	rdb *redis.Client
}

func NewAuthZRepo(fga *openfga.Client, rdb *redis.Client) biz.AuthZRepo {
	if fga == nil {
		return nil
	}
	return &authzRepo{fga: fga, rdb: rdb}
}

func (r *authzRepo) WriteTuples(ctx context.Context, tuples ...biz.Tuple) error {
	return r.fga.WriteTuples(ctx, toFGATuples(tuples)...)
}

func (r *authzRepo) DeleteTuples(ctx context.Context, tuples ...biz.Tuple) error {
	return r.fga.DeleteTuples(ctx, toFGATuples(tuples)...)
}

func (r *authzRepo) Check(ctx context.Context, userID, relation, objectType, objectID string) (bool, error) {
	return r.fga.CachedCheck(ctx, r.rdb, openfga.DefaultCheckCacheTTL, userID, relation, objectType, objectID)
}

func (r *authzRepo) ListObjects(ctx context.Context, userID, relation, objectType string) ([]string, error) {
	return r.fga.ListObjects(ctx, userID, relation, objectType)
}

func (r *authzRepo) CachedListObjects(ctx context.Context, ttl time.Duration, userID, relation, objectType string) ([]string, error) {
	return r.fga.CachedListObjects(ctx, r.rdb, ttl, userID, relation, objectType)
}

func (r *authzRepo) InvalidateListObjects(ctx context.Context, userID, relation, objectType string) {
	openfga.InvalidateListObjects(ctx, r.rdb, userID, relation, objectType)
}

func toFGATuples(tuples []biz.Tuple) []openfga.Tuple {
	out := make([]openfga.Tuple, len(tuples))
	for i, t := range tuples {
		out[i] = openfga.Tuple{User: t.User, Relation: t.Relation, Object: t.Object}
	}
	return out
}

package middleware

import (
	"time"

	"github.com/go-kratos/kratos/v2/middleware"

	iamv1 "github.com/Servora-Kit/servora/api/gen/go/iam/service/v1"
	pkgauthz "github.com/Servora-Kit/servora/pkg/authz"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/Servora-Kit/servora/pkg/redis"
)

// AuthzOption is an alias for pkgauthz.Option so callers don't need to import pkg/authz directly.
type AuthzOption = pkgauthz.Option

// WithFGAClient sets the OpenFGA client used for authorization checks.
func WithFGAClient(c *openfga.Client) AuthzOption { return pkgauthz.WithFGAClient(c) }

// WithAuthzCache enables Redis caching of authorization check results.
func WithAuthzCache(rdb *redis.Client, ttl time.Duration) AuthzOption {
	return pkgauthz.WithAuthzCache(rdb, ttl)
}

// WithAuthzRules converts IAM-generated AuthzRuleEntry map to pkg/authz AuthzRule map.
func WithAuthzRules(rules map[string]iamv1.AuthzRuleEntry) AuthzOption {
	converted := make(map[string]pkgauthz.AuthzRule, len(rules))
	for op, r := range rules {
		converted[op] = pkgauthz.AuthzRule{
			Mode:       r.Mode,
			Relation:   r.Relation,
			ObjectType: r.ObjectType,
			IDField:    r.IDField,
		}
	}
	return pkgauthz.WithAuthzRules(converted)
}

// Authz returns a Kratos middleware that performs OpenFGA authorization checks
// based on proto-declared rules. See pkg/authz for full behavior documentation.
func Authz(opts ...AuthzOption) middleware.Middleware { return pkgauthz.Authz(opts...) }

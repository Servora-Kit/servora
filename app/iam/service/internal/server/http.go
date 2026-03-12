package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	iamv1 "github.com/Servora-Kit/servora/api/gen/go/iam/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/biz"
	"github.com/Servora-Kit/servora/app/iam/service/internal/service"
	"github.com/Servora-Kit/servora/pkg/governance/telemetry"
	"github.com/Servora-Kit/servora/pkg/health"
	"github.com/Servora-Kit/servora/pkg/jwks"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/Servora-Kit/servora/pkg/redis"
	"github.com/Servora-Kit/servora/pkg/transport/server/http"
	svrmw "github.com/Servora-Kit/servora/pkg/transport/server/middleware"
)

type HTTPMiddleware []middleware.Middleware

func NewHTTPMiddleware(
	trace *conf.Trace,
	m *telemetry.Metrics,
	l logger.Logger,
	km *jwks.KeyManager,
	fga *openfga.Client,
	rdb *redis.Client,
	platID biz.PlatformRootID,
) HTTPMiddleware {
	ms := svrmw.NewChainBuilder(logger.With(l, logger.WithModule("http/server/iam-service"))).
		WithTrace(trace).
		WithMetrics(m).
		Build()

	publicWhitelist := svrmw.NewWhiteList(svrmw.Exact,
		iamv1.OperationAuthServiceLoginByEmailPassword,
		iamv1.OperationAuthServiceRefreshToken,
		iamv1.OperationAuthServiceSignupByEmail,
		iamv1.OperationTestServiceTest,
		iamv1.OperationTestServiceHello,
	)

	authn := svrmw.Authn(svrmw.WithVerifier(km.Verifier()))

	authzRules := convertAuthzRules(iamv1.AuthzRules)
	authzOpts := []svrmw.AuthzOption{
		svrmw.WithFGAClient(fga),
		svrmw.WithAuthzRules(authzRules),
		svrmw.WithPlatformRootID(string(platID)),
	}
	if rdb != nil {
		authzOpts = append(authzOpts, svrmw.WithAuthzCache(rdb, openfga.DefaultCheckCacheTTL))
	}
	authz := svrmw.Authz(authzOpts...)

	ms = append(ms,
		selector.Server(authn).
			Match(publicWhitelist.MatchFunc()).
			Build(),
		authz,
	)

	return ms
}

func convertAuthzRules(src map[string]iamv1.AuthzRuleEntry) map[string]svrmw.AuthzRuleEntry {
	dst := make(map[string]svrmw.AuthzRuleEntry, len(src))
	for op, r := range src {
		dst[op] = svrmw.AuthzRuleEntry{
			Mode:       r.Mode,
			Relation:   r.Relation,
			ObjectType: r.ObjectType,
			IDField:    r.IDField,
		}
	}
	return dst
}

func NewHealthHandler(redisClient *redis.Client) *health.Handler {
	return health.NewHandlerWithDefaults(health.DefaultDeps{
		Redis: redisClient,
	})
}

func NewHTTPServer(
	c *conf.Server,
	appCfg *conf.App,
	mw HTTPMiddleware,
	mtc *telemetry.Metrics,
	l logger.Logger,
	h *health.Handler,
	km *jwks.KeyManager,
	auth *service.AuthService,
	user *service.UserService,
	test *service.TestService,
	org *service.OrganizationService,
	proj *service.ProjectService,
) *khttp.Server {
	hlog := logger.With(l, logger.WithModule("http/server/iam-service"))

	issuerURL := ""
	if appCfg.Jwt != nil {
		issuerURL = appCfg.Jwt.IssuerUrl
	}
	if issuerURL == "" {
		addr := "http://localhost:8000"
		if c != nil && c.Http != nil && c.Http.Addr != "" {
			addr = "http://" + c.Http.Addr
		}
		issuerURL = addr
	}

	opts := []http.ServerOption{
		http.WithLogger(hlog),
		http.WithMiddleware(mw...),
		http.WithMetrics(mtc),
		http.WithHealthCheck(h),
		http.WithServices(
			func(s *khttp.Server) { iamv1.RegisterAuthServiceHTTPServer(s, auth) },
			func(s *khttp.Server) { iamv1.RegisterUserServiceHTTPServer(s, user) },
			func(s *khttp.Server) { iamv1.RegisterTestServiceHTTPServer(s, test) },
			func(s *khttp.Server) { iamv1.RegisterOrganizationServiceHTTPServer(s, org) },
			func(s *khttp.Server) { iamv1.RegisterProjectServiceHTTPServer(s, proj) },
			func(s *khttp.Server) {
				s.Handle("/.well-known/jwks.json", jwks.NewJWKSHandler(km))
				s.Handle("/.well-known/openid-configuration", jwks.NewOIDCDiscoveryHandler(issuerURL))
			},
		),
	}
	if c != nil && c.Http != nil {
		opts = append(opts, http.WithConfig(c.Http))
		opts = append(opts, http.WithCORS(c.Http.Cors))
	}

	return http.NewServer(opts...)
}

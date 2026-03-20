package server

import (
	nethttp "net/http"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	iamv1 "github.com/Servora-Kit/servora/api/gen/go/iam/service/v1"
	"github.com/Servora-Kit/servora/app/iam/service/internal/assets"
	"github.com/Servora-Kit/servora/app/iam/service/internal/oidc"
	"github.com/Servora-Kit/servora/app/iam/service/internal/service"
	"github.com/Servora-Kit/servora/pkg/authn"
	"github.com/Servora-Kit/servora/pkg/authz"
	"github.com/Servora-Kit/servora/pkg/cap"
	"github.com/Servora-Kit/servora/pkg/governance/telemetry"
	"github.com/Servora-Kit/servora/pkg/health"
	"github.com/Servora-Kit/servora/pkg/jwks"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/Servora-Kit/servora/pkg/openfga"
	"github.com/Servora-Kit/servora/pkg/redis"
	"github.com/Servora-Kit/servora/pkg/swagger"
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
) HTTPMiddleware {
	ms := svrmw.NewChainBuilder(logger.With(l, "http/server/iam")).
		WithTrace(trace).
		WithMetrics(m).
		Build()

	publicWhitelist := svrmw.NewWhiteList(svrmw.Exact,
		iamv1.OperationAuthnServiceLoginByEmailPassword,
		iamv1.OperationAuthnServiceRefreshToken,
		iamv1.OperationAuthnServiceSignupByEmail,
		iamv1.OperationAuthnServiceRequestEmailVerification,
		iamv1.OperationAuthnServiceVerifyEmail,
		iamv1.OperationAuthnServiceRequestPasswordReset,
		iamv1.OperationAuthnServiceResetPassword,
		cap.OperationCapChallenge,
		cap.OperationCapRedeem,
	)

	authnMw := authn.Authn(authn.WithVerifier(km.Verifier()))

	authzOpts := []authz.Option{
		authz.WithFGAClient(fga),
		authz.WithAuthzRules(iamv1.AuthzRules),
	}
	if rdb != nil {
		authzOpts = append(authzOpts, authz.WithAuthzCache(rdb, openfga.DefaultCheckCacheTTL))
	}
	authzMw := authz.Authz(authzOpts...)

	ms = append(ms,
		selector.Server(authnMw).
			Match(publicWhitelist.MatchFunc()).
			Build(),
		authzMw,
	)

	return ms
}

func NewHealthHandler(redisClient *redis.Client, drv *entsql.Driver) *health.Handler {
	return health.NewHandlerWithDefaults(health.DefaultDeps{
		Redis: redisClient,
		DB:    drv.DB(),
	})
}

func NewHTTPServer(
	c *conf.Server,
	appCfg *conf.App,
	mw HTTPMiddleware,
	mtc *telemetry.Metrics,
	l logger.Logger,
	h *health.Handler,
	authn *service.AuthnService,
	user *service.UserService,
	app *service.ApplicationService,
	capSvc *cap.Cap,
	oidcProvider *op.Provider,
	loginHandler *oidc.LoginHandler,
	loginCompleteHandler *oidc.LoginCompleteHandler,
) *khttp.Server {
	hlog := logger.With(l, "http/server/iam")

	opts := []http.ServerOption{
		http.WithLogger(hlog),
		http.WithMiddleware(mw...),
		http.WithMetrics(mtc),
		http.WithHealthCheck(h),
		http.WithSwagger(assets.OpenAPIData, swagger.WithTitle("IAM API")),
		http.WithServices(
			forwardAuthVerify(authn),
			func(s *khttp.Server) { cap.Register(s, capSvc) },
			func(s *khttp.Server) { iamv1.RegisterAuthnServiceHTTPServer(s, authn) },
			func(s *khttp.Server) { iamv1.RegisterUserServiceHTTPServer(s, user) },
			func(s *khttp.Server) { iamv1.RegisterApplicationServiceHTTPServer(s, app) },
			func(s *khttp.Server) { oidc.Register(s, oidcProvider, loginHandler, loginCompleteHandler) },
		),
	}
	if c != nil && c.Http != nil {
		opts = append(opts, http.WithConfig(c.Http))
		opts = append(opts, http.WithCORS(c.Http.Cors))
	}

	return http.NewServer(opts...)
}

// forwardAuthVerify 注册 GET/HEAD /v1/auth/verify 供网关 ForwardAuth 调用。
func forwardAuthVerify(authSvc *service.AuthnService) func(s *khttp.Server) {
	return func(s *khttp.Server) {
		s.Handle("/v1/auth/verify", nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			if r.Method != nethttp.MethodGet && r.Method != nethttp.MethodHead {
				w.WriteHeader(nethttp.StatusMethodNotAllowed)
				return
			}
			userID, err := authSvc.VerifyAuthorizationHeader(r.Header.Get("Authorization"))
			if err != nil {
				w.WriteHeader(nethttp.StatusUnauthorized)
				return
			}
			w.Header().Set("X-User-ID", userID)
			w.WriteHeader(nethttp.StatusNoContent)
		}))
	}
}

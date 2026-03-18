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
	iammw "github.com/Servora-Kit/servora/app/iam/service/internal/server/middleware"
	"github.com/Servora-Kit/servora/app/iam/service/internal/oidc"
	"github.com/Servora-Kit/servora/app/iam/service/internal/service"
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
	ms := svrmw.NewChainBuilder(logger.With(l, logger.WithModule("http/server/iam-service"))).
		WithTrace(trace).
		WithMetrics(m).
		Build()

	publicWhitelist := svrmw.NewWhiteList(svrmw.Exact,
		// 登录/注册/刷新 token —— 用户尚未持有 JWT，无需鉴权
		iamv1.OperationAuthnServiceLoginByEmailPassword,
		iamv1.OperationAuthnServiceRefreshToken,
		iamv1.OperationAuthnServiceSignupByEmail,
		// 邮箱验证/密码重置 —— 用户可能未登录，且业务逻辑本身已防信息泄露
		iamv1.OperationAuthnServiceRequestEmailVerification,
		iamv1.OperationAuthnServiceVerifyEmail,
		iamv1.OperationAuthnServiceRequestPasswordReset,
		iamv1.OperationAuthnServiceResetPassword,
		// Cap 人机验证 —— 在注册页面使用，用户此时尚未登录
		cap.OperationCapChallenge,
		cap.OperationCapRedeem,
		// 测试接口
		iamv1.OperationTestServiceTest,
		iamv1.OperationTestServiceHello,
	)

	authn := iammw.Authn(iammw.WithVerifier(km.Verifier()))

	authzOpts := []iammw.AuthzOption{
		iammw.WithFGAClient(fga),
		iammw.WithAuthzRules(iamv1.AuthzRules),
	}
	if rdb != nil {
		authzOpts = append(authzOpts, iammw.WithAuthzCache(rdb, openfga.DefaultCheckCacheTTL))
	}
	authz := iammw.Authz(authzOpts...)

	ms = append(ms,
		selector.Server(authn).
			Match(publicWhitelist.MatchFunc()).
			Build(),
		svrmw.ScopeFromHeaders(),
		authz,
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
	test *service.TestService,
	org *service.OrganizationService,
	tenant *service.TenantService,
	app *service.ApplicationService,
	rbac *service.RbacService,
	position *service.PositionService,
	dict *service.DictService,
	capSvc *cap.Cap,
	oidcProvider *op.Provider,
	loginHandler *oidc.LoginHandler,
	loginCompleteHandler *oidc.LoginCompleteHandler,
) *khttp.Server {
	hlog := logger.With(l, logger.WithModule("http/server/iam-service"))

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
			func(s *khttp.Server) { iamv1.RegisterTestServiceHTTPServer(s, test) },
			func(s *khttp.Server) { iamv1.RegisterOrganizationServiceHTTPServer(s, org) },
			func(s *khttp.Server) { iamv1.RegisterTenantServiceHTTPServer(s, tenant) },
			func(s *khttp.Server) { iamv1.RegisterApplicationServiceHTTPServer(s, app) },
			func(s *khttp.Server) { iamv1.RegisterRbacServiceHTTPServer(s, rbac) },
			func(s *khttp.Server) { iamv1.RegisterPositionServiceHTTPServer(s, position) },
			func(s *khttp.Server) { iamv1.RegisterDictServiceHTTPServer(s, dict) },
			func(s *khttp.Server) { oidc.Register(s, oidcProvider, loginHandler, loginCompleteHandler) },
		),
	}
	if c != nil && c.Http != nil {
		opts = append(opts, http.WithConfig(c.Http))
		opts = append(opts, http.WithCORS(c.Http.Cors))
	}

	return http.NewServer(opts...)
}

// forwardAuthVerify 注册 GET/HEAD /v1/auth/verify，供 Traefik 等网关 ForwardAuth 调用。
// 校验 Authorization Bearer token，成功 204 + X-User-ID，失败 401。
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

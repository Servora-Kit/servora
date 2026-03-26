package http

import (
	"net/http"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/platform/health"
	"github.com/Servora-Kit/servora/platform/swagger"
	"github.com/Servora-Kit/servora/transport/server"
	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

type Registrar func(*khttp.Server)

type ServerOption func(*serverOptions)

type serverOptions struct {
	conf           *conf.Server_HTTP
	logger         log.Logger
	middleware     []middleware.Middleware
	cors           *conf.CORS
	metricsHandler http.Handler
	registrars     []Registrar
	healthHandler  *health.Handler
	swaggerSpec    []byte
	swaggerOpts    []swagger.Option
}

func WithConfig(c *conf.Server_HTTP) ServerOption {
	return func(o *serverOptions) {
		o.conf = c
	}
}

func WithLogger(l log.Logger) ServerOption {
	return func(o *serverOptions) {
		o.logger = l
	}
}

func WithMiddleware(mw ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) {
		o.middleware = mw
	}
}

func WithCORS(c *conf.CORS) ServerOption {
	return func(o *serverOptions) {
		o.cors = c
	}
}

func WithMetrics(m *telemetry.Metrics) ServerOption {
	return func(o *serverOptions) {
		if m != nil {
			o.metricsHandler = m.Handler
		}
	}
}

func WithServices(registrars ...Registrar) ServerOption {
	return func(o *serverOptions) {
		o.registrars = registrars
	}
}

// WithHealthCheck 启用健康探针端点。
// 注册 GET /healthz (liveness) 和 GET /readyz (readiness) 路由。
func WithHealthCheck(h *health.Handler) ServerOption {
	return func(o *serverOptions) {
		o.healthHandler = h
	}
}

// WithSwagger 启用 Swagger UI 文档端点。
// 注册 GET /docs/ (UI 页面) 和 GET /docs/openapi.yaml (原始 spec) 路由。
func WithSwagger(specData []byte, opts ...swagger.Option) ServerOption {
	return func(o *serverOptions) {
		o.swaggerSpec = specData
		o.swaggerOpts = opts
	}
}

func NewServer(opts ...ServerOption) *khttp.Server {
	o := &serverOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var serverOpts []khttp.ServerOption

	if o.logger != nil {
		serverOpts = append(serverOpts, khttp.Logger(o.logger))
	}
	if len(o.middleware) > 0 {
		serverOpts = append(serverOpts, khttp.Middleware(o.middleware...))
	}

	if o.conf != nil {
		if o.conf.Network != "" {
			serverOpts = append(serverOpts, khttp.Network(o.conf.Network))
		}
		if o.conf.Addr != "" {
			serverOpts = append(serverOpts, khttp.Address(o.conf.Addr))
		}
		if o.conf.Timeout != nil {
			serverOpts = append(serverOpts, khttp.Timeout(o.conf.Timeout.AsDuration()))
		}
		if o.conf.Tls != nil && o.conf.Tls.Enable {
			tlsCfg := server.MustLoadTLS(o.conf.Tls)
			serverOpts = append(serverOpts, khttp.TLSConfig(tlsCfg))
		}
	}

	if svrmw.IsEnabled(o.cors) {
		serverOpts = append(serverOpts, khttp.Filter(svrmw.Middleware(o.cors)))
	}

	srv := khttp.NewServer(serverOpts...)

	if o.metricsHandler != nil {
		srv.Handle("/metrics", o.metricsHandler)
	}

	if o.healthHandler != nil {
		srv.HandleFunc("/healthz", o.healthHandler.LivenessHandler())
		srv.HandleFunc("/readyz", o.healthHandler.ReadinessHandler())
	}

	if len(o.swaggerSpec) > 0 {
		swagger.Register(srv, o.swaggerSpec, o.swaggerOpts...)
	}

	for _, reg := range o.registrars {
		reg(srv)
	}

	return srv
}

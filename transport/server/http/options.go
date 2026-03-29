package http

import (
	"net/http"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/platform/health"
	"github.com/Servora-Kit/servora/platform/swagger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
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

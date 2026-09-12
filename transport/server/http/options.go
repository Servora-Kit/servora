package http

import (
	"net/http"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
	"github.com/Servora-Kit/servora/obs/metrics"
	"github.com/Servora-Kit/servora/transport/server/http/health"
	"github.com/go-kratos/kratos/v3/middleware"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

type Registrar func(*khttp.Server)

type ServerOption func(*serverOptions)

type serverOptions struct {
	conf           *corev1.Server_HTTP
	middleware     []middleware.Middleware
	filters        []khttp.FilterFunc
	cors           *corsv1.CORS
	metricsHandler http.Handler
	registrars     []Registrar
	healthHandler  *health.Handler
}

func WithConfig(c *corev1.Server_HTTP) ServerOption {
	return func(o *serverOptions) {
		o.conf = c
	}
}

func WithMiddleware(mw ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) {
		o.middleware = mw
	}
}

// WithFilter 按声明顺序安装原生 HTTP 中间件，首个位于最外层。
// Filter 覆盖所有 handler；启用的内置 CORS 在这些 Filter 之外处理预检。
func WithFilter(filters ...khttp.FilterFunc) ServerOption {
	return func(o *serverOptions) {
		o.filters = append(o.filters, filters...)
	}
}

func WithCORS(c *corsv1.CORS) ServerOption {
	return func(o *serverOptions) {
		o.cors = c
	}
}

func WithMetrics(m *metrics.Metrics) ServerOption {
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

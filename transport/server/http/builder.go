package http

import (
	"context"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/platform/health"
	"github.com/Servora-Kit/servora/platform/swagger"
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// Builder 提供面向调用方的 HTTP server DSL，隐藏 runtime graph 细节。
type Builder struct {
	config        *conf.Server_HTTP
	logger        log.Logger
	middleware    []middleware.Middleware
	cors          *conf.CORS
	metrics       *telemetry.Metrics
	healthHandler *health.Handler
	swaggerSpec   []byte
	swaggerOpts   []swagger.Option
	registrars    []Registrar
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithConfig(c *conf.Server_HTTP) *Builder {
	b.config = c
	return b
}

func (b *Builder) WithLogger(l log.Logger) *Builder {
	b.logger = l
	return b
}

func (b *Builder) WithMiddleware(mw ...middleware.Middleware) *Builder {
	b.middleware = append(b.middleware, mw...)
	return b
}

func (b *Builder) WithCORS(cors *conf.CORS) *Builder {
	b.cors = cors
	return b
}

func (b *Builder) WithMetrics(metrics *telemetry.Metrics) *Builder {
	b.metrics = metrics
	return b
}

func (b *Builder) WithHealthCheck(h *health.Handler) *Builder {
	b.healthHandler = h
	return b
}

func (b *Builder) WithSwagger(spec []byte, opts ...swagger.Option) *Builder {
	b.swaggerSpec = spec
	b.swaggerOpts = append([]swagger.Option(nil), opts...)
	return b
}

func (b *Builder) WithServices(registrars ...Registrar) *Builder {
	b.registrars = append(b.registrars, registrars...)
	return b
}

func (b *Builder) Build(ctx context.Context) (*khttp.Server, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cors := b.cors
	if cors == nil && b.config != nil {
		cors = b.config.Cors
	}

	in := runtime.ServerBuildInput{
		Config: &ServerConfig{
			HTTP:           b.config,
			CORS:           cors,
			Metrics:        b.metrics,
			HealthHandler:  b.healthHandler,
			SwaggerSpec:    b.swaggerSpec,
			SwaggerOptions: b.swaggerOpts,
		},
		Logger:     b.logger,
		Middleware: b.middleware,
	}
	if len(b.registrars) > 0 {
		in.Registrars = make([]any, 0, len(b.registrars))
		for _, reg := range b.registrars {
			if reg == nil {
				continue
			}
			in.Registrars = append(in.Registrars, reg)
		}
	}

	raw, err := (&Plugin{}).Build(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("build http server from plugin: %w", err)
	}

	srv, ok := raw.(*khttp.Server)
	if !ok {
		return nil, fmt.Errorf("unexpected http server type: %T", raw)
	}

	return srv, nil
}

func (b *Builder) MustBuild() *khttp.Server {
	srv, err := b.Build(context.Background())
	if err != nil {
		panic(err)
	}
	return srv
}

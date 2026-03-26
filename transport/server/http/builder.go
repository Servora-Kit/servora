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
	extraValues   map[string]any
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

// WithExtraValue 允许注入额外 plugin 参数，便于未来协议扩展。
func (b *Builder) WithExtraValue(key string, value any) *Builder {
	if b.extraValues == nil {
		b.extraValues = make(map[string]any)
	}
	b.extraValues[key] = value
	return b
}

// WithExtraValues 批量注入额外 plugin 参数，后写入值覆盖先前同名键。
func (b *Builder) WithExtraValues(values map[string]any) *Builder {
	if len(values) == 0 {
		return b
	}
	if b.extraValues == nil {
		b.extraValues = make(map[string]any, len(values))
	}
	for k, v := range values {
		b.extraValues[k] = v
	}
	return b
}

func (b *Builder) Build(ctx context.Context) (*khttp.Server, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	in := runtime.ServerBuildInput{
		Config:     b.config,
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
	in.ExtraValues = b.buildExtraValues()

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

func (b *Builder) buildExtraValues() map[string]any {
	extra := make(map[string]any, len(b.extraValues)+5)
	for k, v := range b.extraValues {
		extra[k] = v
	}

	if b.cors != nil {
		extra[ExtraKeyCORS] = b.cors
	} else if b.config != nil && b.config.Cors != nil {
		extra[ExtraKeyCORS] = b.config.Cors
	}

	if b.metrics != nil {
		extra[ExtraKeyMetrics] = b.metrics
	}
	if b.healthHandler != nil {
		extra[ExtraKeyHealthHandler] = b.healthHandler
	}
	if len(b.swaggerSpec) > 0 {
		extra[ExtraKeySwaggerSpec] = b.swaggerSpec
		if len(b.swaggerOpts) > 0 {
			extra[ExtraKeySwaggerOptions] = b.swaggerOpts
		}
	}

	if len(extra) == 0 {
		return nil
	}
	return extra
}

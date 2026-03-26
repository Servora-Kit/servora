package http

import (
	"context"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/telemetry"
	"github.com/Servora-Kit/servora/platform/health"
	"github.com/Servora-Kit/servora/platform/swagger"
	"github.com/Servora-Kit/servora/transport/runtime"
)

const (
	Type = "http"

	// ExtraKeyCORS 允许 runtime graph 向 HTTP plugin 注入 CORS 配置。
	ExtraKeyCORS = "cors"
	// ExtraKeyMetrics 允许 runtime graph 向 HTTP plugin 注入 metrics handler。
	ExtraKeyMetrics = "metrics"
	// ExtraKeyHealthHandler 允许 runtime graph 向 HTTP plugin 注入健康检查处理器。
	ExtraKeyHealthHandler = "health_handler"
	// ExtraKeySwaggerSpec 允许 runtime graph 向 HTTP plugin 注入 swagger spec。
	ExtraKeySwaggerSpec = "swagger_spec"
	// ExtraKeySwaggerOptions 允许 runtime graph 向 HTTP plugin 注入 swagger options。
	ExtraKeySwaggerOptions = "swagger_options"
)

// Plugin 将 HTTP server 适配到 transport runtime graph。
type Plugin struct{}

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ServerBuildInput) (runtime.Server, error) {
	opts := make([]ServerOption, 0, 8)

	if in.Config != nil {
		cfg, ok := in.Config.(*conf.Server_HTTP)
		if !ok {
			return nil, fmt.Errorf("http plugin expects *conf.Server_HTTP config, got %T", in.Config)
		}
		opts = append(opts, WithConfig(cfg))
	}
	if in.Logger != nil {
		opts = append(opts, WithLogger(in.Logger))
	}
	if len(in.Middleware) > 0 {
		opts = append(opts, WithMiddleware(in.Middleware...))
	}

	registrars := make([]Registrar, 0, len(in.Registrars))
	for _, raw := range in.Registrars {
		if raw == nil {
			continue
		}
		reg, ok := raw.(Registrar)
		if !ok {
			return nil, fmt.Errorf("http plugin expects registrar type %T, got %T", Registrar(nil), raw)
		}
		registrars = append(registrars, reg)
	}
	if len(registrars) > 0 {
		opts = append(opts, WithServices(registrars...))
	}

	if len(in.ExtraValues) > 0 {
		if raw, ok := in.ExtraValues[ExtraKeyCORS]; ok && raw != nil {
			corsCfg, ok := raw.(*conf.CORS)
			if !ok {
				return nil, fmt.Errorf("http plugin expects *conf.CORS for %q, got %T", ExtraKeyCORS, raw)
			}
			opts = append(opts, WithCORS(corsCfg))
		}
		if raw, ok := in.ExtraValues[ExtraKeyMetrics]; ok && raw != nil {
			metrics, ok := raw.(*telemetry.Metrics)
			if !ok {
				return nil, fmt.Errorf("http plugin expects *telemetry.Metrics for %q, got %T", ExtraKeyMetrics, raw)
			}
			opts = append(opts, WithMetrics(metrics))
		}
		if raw, ok := in.ExtraValues[ExtraKeyHealthHandler]; ok && raw != nil {
			h, ok := raw.(*health.Handler)
			if !ok {
				return nil, fmt.Errorf("http plugin expects *health.Handler for %q, got %T", ExtraKeyHealthHandler, raw)
			}
			opts = append(opts, WithHealthCheck(h))
		}
		if raw, ok := in.ExtraValues[ExtraKeySwaggerSpec]; ok && raw != nil {
			spec, ok := raw.([]byte)
			if !ok {
				return nil, fmt.Errorf("http plugin expects []byte for %q, got %T", ExtraKeySwaggerSpec, raw)
			}
			swaggerOpts, err := parseSwaggerOptions(in.ExtraValues)
			if err != nil {
				return nil, err
			}
			opts = append(opts, WithSwagger(spec, swaggerOpts...))
		}
	}

	return NewServer(opts...), nil
}

func parseSwaggerOptions(extra map[string]any) ([]swagger.Option, error) {
	if len(extra) == 0 {
		return nil, nil
	}
	raw, ok := extra[ExtraKeySwaggerOptions]
	if !ok || raw == nil {
		return nil, nil
	}
	opts, ok := raw.([]swagger.Option)
	if !ok {
		return nil, fmt.Errorf("http plugin expects []swagger.Option for %q, got %T", ExtraKeySwaggerOptions, raw)
	}
	return opts, nil
}

package http

import (
	"context"
	"fmt"

	"github.com/Servora-Kit/servora/transport/runtime"
)

const Type = "http"

// Plugin 将 HTTP server 适配到 transport runtime graph。
type Plugin struct{}

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ServerBuildInput) (runtime.Server, error) {
	opts := make([]ServerOption, 0, 8)

	if in.Config != nil {
		cfg, ok := in.Config.(*ServerConfig)
		if !ok {
			return nil, fmt.Errorf("http plugin expects *http.ServerConfig, got %T", in.Config)
		}
		if cfg.HTTP != nil {
			opts = append(opts, WithConfig(cfg.HTTP))
		}
		if cfg.CORS != nil {
			opts = append(opts, WithCORS(cfg.CORS))
		} else if cfg.HTTP != nil && cfg.HTTP.Cors != nil {
			opts = append(opts, WithCORS(cfg.HTTP.Cors))
		}
		if cfg.Metrics != nil {
			opts = append(opts, WithMetrics(cfg.Metrics))
		}
		if cfg.HealthHandler != nil {
			opts = append(opts, WithHealthCheck(cfg.HealthHandler))
		}
		if len(cfg.SwaggerSpec) > 0 {
			opts = append(opts, WithSwagger(cfg.SwaggerSpec, cfg.SwaggerOptions...))
		}
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

	return NewServer(opts...), nil
}

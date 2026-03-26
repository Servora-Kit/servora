package grpc

import (
	"context"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/runtime"
)

// Plugin 将 gRPC server 适配到 transport runtime graph。
type Plugin struct{}

const Type = "grpc"

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ServerBuildInput) (runtime.Server, error) {
	opts := make([]ServerOption, 0, 4)

	if in.Config != nil {
		cfg, ok := in.Config.(*conf.Server_GRPC)
		if !ok {
			return nil, fmt.Errorf("grpc plugin expects *conf.Server_GRPC config, got %T", in.Config)
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
			return nil, fmt.Errorf("grpc plugin expects registrar type %T, got %T", Registrar(nil), raw)
		}
		registrars = append(registrars, reg)
	}
	if len(registrars) > 0 {
		opts = append(opts, WithServices(registrars...))
	}

	return NewServer(opts...), nil
}

package grpc

import (
	"context"
	"fmt"
	"strings"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
)

type Plugin struct{}

const Type = "grpc"

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ClientBuildInput) (runtime.ClientFactory, error) {
	return &factory{
		grpcClients: BuildClientConfigIndex(in.Data),
		discovery:   in.Discovery,
		logger:      in.Logger,
		middleware:  in.Middleware,
	}, nil
}

type factory struct {
	grpcClients map[string]*conf.Data_Client_GRPC
	discovery   registry.Discovery
	logger      logger.Logger
	middleware  []middleware.Middleware
}

func (f *factory) Dial(ctx context.Context, in runtime.ClientDialInput) (runtime.Connection, error) {
	target := strings.TrimSpace(in.Target)
	if target == "" {
		return nil, fmt.Errorf("grpc dial target is empty")
	}
	grpcConn, err := createConnection(ctx, target, f.grpcClients, f.discovery, f.logger, f.middleware)
	if err != nil {
		return nil, err
	}
	return NewConnection(grpcConn), nil
}

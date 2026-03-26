package grpc

import (
	"context"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/go-kratos/kratos/v2/registry"
)

type Plugin struct{}

const Type = "grpc"

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ClientBuildInput) (runtime.ClientFactory, error) {
	return &factory{
		grpcClients: BuildClientConfigIndex(in.Data),
		traceCfg:    in.Trace,
		discovery:   in.Discovery,
		logger:      in.Logger,
	}, nil
}

type factory struct {
	grpcClients map[string]*conf.Data_Client_GRPC
	traceCfg    *conf.Trace
	discovery   registry.Discovery
	logger      logger.Logger
}

func (f *factory) CreateConn(ctx context.Context, serviceName string) (runtime.Connection, error) {
	grpcConn, err := createConnection(ctx, serviceName, f.grpcClients, f.traceCfg, f.discovery, f.logger)
	if err != nil {
		return nil, err
	}
	return NewConnection(grpcConn), nil
}

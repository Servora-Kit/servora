package client

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/horonlee/servora/api/gen/go/conf/v1"
	"github.com/horonlee/servora/pkg/logger"
)

type client struct {
	dataCfg   *conf.Data
	traceCfg  *conf.Trace
	discovery registry.Discovery
	logger    logger.Logger
}

func NewClient(
	dataCfg *conf.Data,
	traceCfg *conf.Trace,
	discovery registry.Discovery,
	l logger.Logger,
) (Client, error) {
	return &client{
		dataCfg:   dataCfg,
		traceCfg:  traceCfg,
		discovery: discovery,
		logger:    logger.With(l, logger.WithModule("client/pkg")),
	}, nil
}

func (c *client) CreateConn(ctx context.Context, connType ConnType, serviceName string) (Connection, error) {
	switch connType {
	case GRPC:
		return c.createGrpcConn(ctx, serviceName)
	default:
		return nil, fmt.Errorf("unsupported connection type: %s", connType)
	}
}

func (c *client) createGrpcConn(ctx context.Context, serviceName string) (Connection, error) {
	grpcConn, err := createGrpcConnection(ctx, serviceName, c.dataCfg, c.traceCfg, c.discovery, c.logger)
	if err != nil {
		return nil, err
	}

	return NewGrpcConn(grpcConn), nil
}

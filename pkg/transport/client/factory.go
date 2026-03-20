package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	"github.com/Servora-Kit/servora/pkg/logger"
	"github.com/go-kratos/kratos/v2/registry"
)

type client struct {
	dataCfg     *conf.Data
	traceCfg    *conf.Trace
	discovery   registry.Discovery
	logger      logger.Logger
	grpcClients map[string]*conf.Data_Client_GRPC
}

func NewClient(
	dataCfg *conf.Data,
	traceCfg *conf.Trace,
	discovery registry.Discovery,
	l logger.Logger,
) (Client, error) {
	return &client{
		dataCfg:     dataCfg,
		traceCfg:    traceCfg,
		discovery:   discovery,
		logger:      logger.With(l, "client/pkg"),
		grpcClients: initGRPCClients(dataCfg),
	}, nil
}

// initGRPCClients 预构建 gRPC 客户端配置索引，避免热路径重复遍历配置列表。
func initGRPCClients(dataCfg *conf.Data) map[string]*conf.Data_Client_GRPC {
	if dataCfg == nil || dataCfg.Client == nil {
		return nil
	}

	grpcConfigs := dataCfg.Client.GetGrpc()
	if len(grpcConfigs) == 0 {
		return nil
	}

	index := make(map[string]*conf.Data_Client_GRPC, len(grpcConfigs))
	for _, grpcCfg := range grpcConfigs {
		if grpcCfg == nil {
			continue
		}

		serviceName := strings.TrimSpace(grpcCfg.GetServiceName())
		if serviceName == "" {
			continue
		}

		index[serviceName] = grpcCfg
	}

	if len(index) == 0 {
		return nil
	}

	return index
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
	grpcConn, err := createGrpcConnection(ctx, serviceName, c.grpcClients, c.traceCfg, c.discovery, c.logger)
	if err != nil {
		return nil, err
	}

	return NewGrpcConn(grpcConn), nil
}

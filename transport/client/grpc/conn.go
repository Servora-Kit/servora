package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	climw "github.com/Servora-Kit/servora/transport/client/middleware"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
	sharedtls "github.com/Servora-Kit/servora/transport/shared/tls"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	klogging "github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	gogrpc "google.golang.org/grpc"
)

// Connection gRPC 连接封装，实现 runtime.Connection。
type Connection struct {
	conn gogrpc.ClientConnInterface
	ref  int32
	mu   sync.RWMutex
}

func NewConnection(conn gogrpc.ClientConnInterface) *Connection {
	return &Connection{conn: conn}
}

func (g *Connection) Value() any {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.conn
}

func (g *Connection) Close() error {
	newRef := atomic.AddInt32(&g.ref, -1)
	if newRef < 0 {
		panic(fmt.Sprintf("negative ref: %d", newRef))
	}
	return nil
}

func (g *Connection) IsHealthy() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.conn != nil
}

// BuildClientConfigIndex 预构建 gRPC 客户端配置索引，避免热路径重复遍历配置列表。
func BuildClientConfigIndex(dataCfg *conf.Data) map[string]*conf.Data_Client_GRPC {
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

func createConnection(
	ctx context.Context,
	serviceName string,
	grpcConfigs map[string]*conf.Data_Client_GRPC,
	traceCfg *conf.Trace,
	discovery registry.Discovery,
	l logger.Logger,
) (gogrpc.ClientConnInterface, error) {
	setupLogger := logger.NewHelper(l, logger.WithField("operation", "createGrpcConnection"))

	defaultEndpoint := fmt.Sprintf("discovery:///%s", serviceName)
	endpoint, timeout, tlsCfg, configured := resolveConnectionConfig(serviceName, grpcConfigs, defaultEndpoint, 5*time.Second)
	tlsEnabled := tlsCfg != nil && tlsCfg.GetEnable()
	if configured {
		setupLogger.Infof("using configured endpoint: service_name=%s endpoint=%s tls=%t", serviceName, endpoint, tlsEnabled)
	}

	mds := []middleware.Middleware{
		recovery.Recovery(),
		klogging.Client(l),
		circuitbreaker.Client(),
		climw.TokenPropagation(),
	}

	if traceCfg != nil && traceCfg.Endpoint != "" {
		mds = append(mds, tracing.Client())
	}

	opts := []kgrpc.ClientOption{
		kgrpc.WithEndpoint(endpoint),
		kgrpc.WithTimeout(timeout),
		kgrpc.WithMiddleware(mds...),
	}
	if endpoint == defaultEndpoint && discovery != nil {
		opts = append(opts, kgrpc.WithDiscovery(discovery))
	}

	conn, err := dialConnection(ctx, opts, tlsCfg)
	if err != nil {
		setupLogger.Errorf("failed to create grpc client: service_name=%s error=%v", serviceName, err)
		return nil, fmt.Errorf("failed to create grpc client for service %s: %w", serviceName, err)
	}

	setupLogger.Infof("successfully created grpc client: service_name=%s endpoint=%s timeout=%s tls=%t", serviceName, endpoint, timeout.String(), tlsEnabled)
	return conn, nil
}

func dialConnection(ctx context.Context, opts []kgrpc.ClientOption, tlsCfg *conf.TLSConfig) (gogrpc.ClientConnInterface, error) {
	if tlsCfg == nil || !tlsCfg.GetEnable() {
		return kgrpc.DialInsecure(ctx, opts...)
	}

	clientTLSCfg, err := sharedtls.BuildClientTLS(tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("build grpc TLS config: %w", err)
	}

	opts = append(opts, kgrpc.WithTLSConfig(clientTLSCfg))
	return kgrpc.Dial(ctx, opts...)
}

// resolveConnectionConfig 根据服务名解析连接配置，并在缺省时回落到默认端点与超时。
func resolveConnectionConfig(
	serviceName string,
	grpcConfigs map[string]*conf.Data_Client_GRPC,
	defaultEndpoint string,
	defaultTimeout time.Duration,
) (string, time.Duration, *conf.TLSConfig, bool) {
	endpoint := defaultEndpoint
	timeout := defaultTimeout
	var tlsCfg *conf.TLSConfig

	grpcCfg, ok := grpcConfigs[serviceName]
	if !ok || grpcCfg == nil {
		return endpoint, timeout, tlsCfg, false
	}

	timeout = sharedconfig.NormalizeDuration(grpcCfg.GetTimeout(), timeout)
	endpoint = sharedconfig.NormalizeEndpoint(grpcCfg.GetEndpoint(), endpoint)
	if configuredTLS := grpcCfg.GetTls(); configuredTLS != nil {
		tlsCfg = configuredTLS
	}

	return endpoint, timeout, tlsCfg, true
}

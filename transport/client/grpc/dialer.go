package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	logger "github.com/Servora-Kit/servora/obs/logging"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
	sharedtls "github.com/Servora-Kit/servora/transport/shared/tls"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	gogrpc "google.golang.org/grpc"
)

const Type = "grpc"

type Option func(*dialerOptions)

type dialerOptions struct {
	data       *conf.Data
	discovery  registry.Discovery
	logger     logger.Logger
	middleware []middleware.Middleware
}

func WithData(data *conf.Data) Option {
	return func(o *dialerOptions) {
		o.data = data
	}
}

func WithDiscovery(discovery registry.Discovery) Option {
	return func(o *dialerOptions) {
		o.discovery = discovery
	}
}

func WithLogger(l logger.Logger) Option {
	return func(o *dialerOptions) {
		o.logger = l
	}
}

func WithMiddleware(mw ...middleware.Middleware) Option {
	return func(o *dialerOptions) {
		o.middleware = append([]middleware.Middleware(nil), mw...)
	}
}

type Dialer struct {
	grpcClients map[string]*conf.Data_Client_Endpoint
	discovery   registry.Discovery
	logger      logger.Logger
	middleware  []middleware.Middleware
}

func NewDialer(opts ...Option) *Dialer {
	o := dialerOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	grpcClients, err := BuildClientConfigIndex(o.data)
	if err != nil {
		panic(fmt.Sprintf("build grpc client config index: %v", err))
	}

	return &Dialer{
		grpcClients: grpcClients,
		discovery:   o.discovery,
		logger:      o.logger,
		middleware:  o.middleware,
	}
}

func (d *Dialer) Dial(ctx context.Context, target string) (*gogrpc.ClientConn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("grpc dial target is empty")
	}
	return createConnection(ctx, target, d.grpcClients, d.discovery, d.logger, d.middleware)
}

// BuildClientConfigIndex 预构建 gRPC 客户端配置索引，避免热路径重复遍历配置列表。
func BuildClientConfigIndex(dataCfg *conf.Data) (map[string]*conf.Data_Client_Endpoint, error) {
	return sharedconfig.BuildClientEndpointIndex(dataCfg, Type)
}

func createConnection(
	ctx context.Context,
	serviceName string,
	grpcConfigs map[string]*conf.Data_Client_Endpoint,
	discovery registry.Discovery,
	l logger.Logger,
	mds []middleware.Middleware,
) (*gogrpc.ClientConn, error) {
	var setupLogger *logger.Helper
	if l != nil {
		setupLogger = logger.NewHelper(l, logger.WithField("operation", "createGrpcConnection"))
	}

	defaultEndpoint := fmt.Sprintf("discovery:///%s", serviceName)
	endpoint, timeout, tlsCfg, configured := resolveConnectionConfig(serviceName, grpcConfigs, defaultEndpoint, 5*time.Second)
	tlsEnabled := tlsCfg != nil && tlsCfg.GetEnable()
	if configured && setupLogger != nil {
		setupLogger.Infof("using configured endpoint: service_name=%s endpoint=%s tls=%t", serviceName, endpoint, tlsEnabled)
	}

	opts := []kgrpc.ClientOption{
		kgrpc.WithEndpoint(endpoint),
		kgrpc.WithTimeout(timeout),
	}
	if len(mds) > 0 {
		opts = append(opts, kgrpc.WithMiddleware(mds...))
	}
	if endpoint == defaultEndpoint && discovery != nil {
		opts = append(opts, kgrpc.WithDiscovery(discovery))
	}

	conn, err := dialConnection(ctx, opts, tlsCfg)
	if err != nil {
		if setupLogger != nil {
			setupLogger.Errorf("failed to create grpc client: service_name=%s error=%v", serviceName, err)
		}
		return nil, fmt.Errorf("failed to create grpc client for service %s: %w", serviceName, err)
	}

	if setupLogger != nil {
		setupLogger.Infof("successfully created grpc client: service_name=%s endpoint=%s timeout=%s tls=%t", serviceName, endpoint, timeout.String(), tlsEnabled)
	}
	return conn, nil
}

func dialConnection(ctx context.Context, opts []kgrpc.ClientOption, tlsCfg *conf.TLSConfig) (*gogrpc.ClientConn, error) {
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
	grpcConfigs map[string]*conf.Data_Client_Endpoint,
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

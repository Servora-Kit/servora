package grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"log/slog"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	tlspb "github.com/Servora-Kit/servora/api/gen/go/servora/security/tls/v1"
	svrtls "github.com/Servora-Kit/servora/security/tls"
	"github.com/Servora-Kit/servora/transport/client/endpoint"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	gogrpc "google.golang.org/grpc"
)

const Type = "grpc"

type Option func(*dialerOptions)

type dialerOptions struct {
	data       *corev1.Data
	discovery  registry.Discovery
	logger     *slog.Logger
	middleware []middleware.Middleware
}

func WithData(data *corev1.Data) Option {
	return func(o *dialerOptions) {
		o.data = data
	}
}

func WithDiscovery(discovery registry.Discovery) Option {
	return func(o *dialerOptions) {
		o.discovery = discovery
	}
}

func WithLogger(l *slog.Logger) Option {
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
	grpcClients map[string]*corev1.Data_Client_Endpoint
	discovery   registry.Discovery
	logger      *slog.Logger
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
func BuildClientConfigIndex(dataCfg *corev1.Data) (map[string]*corev1.Data_Client_Endpoint, error) {
	return endpoint.IndexByProtocol(dataCfg, Type)
}

func createConnection(
	ctx context.Context,
	serviceName string,
	grpcConfigs map[string]*corev1.Data_Client_Endpoint,
	discovery registry.Discovery,
	l *slog.Logger,
	mds []middleware.Middleware,
) (*gogrpc.ClientConn, error) {
	defaultEndpoint := fmt.Sprintf("discovery:///%s", serviceName)
	ep, timeout, tlsCfg, configured := resolveConnectionConfig(serviceName, grpcConfigs, defaultEndpoint, 5*time.Second)
	tlsEnabled := tlsCfg != nil && tlsCfg.GetEnable()
	if configured && l != nil {
		l.Info("using configured endpoint", "service", serviceName, "endpoint", ep, "tls", tlsEnabled)
	}

	opts := []kgrpc.ClientOption{
		kgrpc.WithEndpoint(ep),
		kgrpc.WithTimeout(timeout),
	}
	if len(mds) > 0 {
		opts = append(opts, kgrpc.WithMiddleware(mds...))
	}
	if ep == defaultEndpoint && discovery != nil {
		opts = append(opts, kgrpc.WithDiscovery(discovery))
	}

	conn, err := dialConnection(ctx, opts, tlsCfg)
	if err != nil {
		if l != nil {
			l.Error("failed to create grpc client", "service", serviceName, "err", err)
		}
		return nil, fmt.Errorf("failed to create grpc client for service %s: %w", serviceName, err)
	}

	if l != nil {
		l.Info("grpc client created", "service", serviceName, "endpoint", ep, "timeout", timeout, "tls", tlsEnabled)
	}
	return conn, nil
}

func dialConnection(ctx context.Context, opts []kgrpc.ClientOption, tlsCfg *tlspb.TLS) (*gogrpc.ClientConn, error) {
	if tlsCfg == nil || !tlsCfg.GetEnable() {
		return kgrpc.DialInsecure(ctx, opts...)
	}

	clientTLSCfg, err := svrtls.BuildClientTLS(tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("build grpc TLS config: %w", err)
	}

	opts = append(opts, kgrpc.WithTLSConfig(clientTLSCfg))
	return kgrpc.Dial(ctx, opts...)
}

// resolveConnectionConfig 根据服务名解析连接配置，并在缺省时回落到默认端点与超时。
func resolveConnectionConfig(
	serviceName string,
	grpcConfigs map[string]*corev1.Data_Client_Endpoint,
	defaultEndpoint string,
	defaultTimeout time.Duration,
) (string, time.Duration, *tlspb.TLS, bool) {
	ep := defaultEndpoint
	timeout := defaultTimeout
	var tlsCfg *tlspb.TLS

	grpcCfg, ok := grpcConfigs[serviceName]
	if !ok || grpcCfg == nil {
		return ep, timeout, tlsCfg, false
	}

	if d := grpcCfg.GetTimeout(); d != nil {
		if v := d.AsDuration(); v > 0 {
			timeout = v
		}
	}
	if v := strings.TrimSpace(grpcCfg.GetEndpoint()); v != "" {
		ep = v
	}
	if configuredTLS := grpcCfg.GetTls(); configuredTLS != nil {
		tlsCfg = configuredTLS
	}

	return ep, timeout, tlsCfg, true
}

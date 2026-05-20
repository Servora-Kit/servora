package http

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"log/slog"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/transport/client/endpoint"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const Type = "http"

type Option func(*dialerOptions)

type dialerOptions struct {
	data       *corev1.Data
	discovery  registry.Discovery
	middleware []middleware.Middleware
	logger     *slog.Logger
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

func WithMiddleware(mw ...middleware.Middleware) Option {
	return func(o *dialerOptions) {
		o.middleware = append([]middleware.Middleware(nil), mw...)
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(o *dialerOptions) {
		o.logger = l
	}
}

type Dialer struct {
	httpClients map[string]*corev1.Data_Client_Endpoint
	discovery   registry.Discovery
	middleware  []middleware.Middleware
	logger      *slog.Logger
}

func NewDialer(opts ...Option) *Dialer {
	o := dialerOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}

	httpClients, err := BuildClientConfigIndex(o.data)
	if err != nil {
		panic(fmt.Sprintf("build http client config index: %v", err))
	}

	return &Dialer{
		httpClients: httpClients,
		discovery:   o.discovery,
		middleware:  o.middleware,
		logger:      o.logger,
	}
}

func (d *Dialer) Dial(ctx context.Context, target string) (*khttp.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	defaultTimeout := 5 * time.Second
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("http dial target is empty")
	}

	defaultEndpoint := resolveDefaultHTTPEndpoint(target)
	ep, timeout, configured := resolveConnectionConfig(target, d.httpClients, defaultEndpoint, defaultTimeout)
	if ep == "" {
		return nil, fmt.Errorf("http endpoint not configured for target: %s", target)
	}
	if d.logger != nil {
		if configured {
			d.logger.Info("using configured http endpoint", "target", target, "endpoint", ep)
		} else {
			d.logger.Info("using direct http endpoint", "target", target, "endpoint", ep)
		}
	}

	opts := []khttp.ClientOption{
		khttp.WithEndpoint(ep),
		khttp.WithTimeout(timeout),
	}
	if len(d.middleware) > 0 {
		opts = append(opts, khttp.WithMiddleware(d.middleware...))
	}
	if strings.HasPrefix(ep, "discovery:///") && d.discovery != nil {
		opts = append(opts, khttp.WithDiscovery(d.discovery))
	}

	client, err := khttp.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("build http client for target %s: %w", target, err)
	}
	return client, nil
}

// BuildClientConfigIndex 预构建 HTTP 客户端配置索引，避免热路径重复遍历配置列表。
func BuildClientConfigIndex(dataCfg *corev1.Data) (map[string]*corev1.Data_Client_Endpoint, error) {
	return endpoint.IndexByProtocol(dataCfg, Type)
}

func resolveConnectionConfig(
	serviceName string,
	httpConfigs map[string]*corev1.Data_Client_Endpoint,
	defaultEndpoint string,
	defaultTimeout time.Duration,
) (string, time.Duration, bool) {
	ep := defaultEndpoint
	timeout := defaultTimeout

	httpCfg, ok := httpConfigs[serviceName]
	if !ok || httpCfg == nil {
		return ep, timeout, false
	}

	if d := httpCfg.GetTimeout(); d != nil {
		if v := d.AsDuration(); v > 0 {
			timeout = v
		}
	}
	if v := strings.TrimSpace(httpCfg.GetEndpoint()); v != "" {
		ep = v
	}

	return ep, timeout, true
}

func resolveDefaultHTTPEndpoint(target string) string {
	if target == "" {
		return ""
	}
	if _, err := url.ParseRequestURI(target); err == nil && strings.Contains(target, "://") {
		return target
	}
	return "discovery:///" + target
}

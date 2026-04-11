package http

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	logger "github.com/Servora-Kit/servora/obs/logging"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const Type = "http"

type Option func(*dialerOptions)

type dialerOptions struct {
	data       *conf.Data
	discovery  registry.Discovery
	middleware []middleware.Middleware
	logger     logger.Logger
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

func WithMiddleware(mw ...middleware.Middleware) Option {
	return func(o *dialerOptions) {
		o.middleware = append([]middleware.Middleware(nil), mw...)
	}
}

func WithLogger(l logger.Logger) Option {
	return func(o *dialerOptions) {
		o.logger = l
	}
}

type Dialer struct {
	httpClients map[string]*conf.Data_Client_Endpoint
	discovery   registry.Discovery
	middleware  []middleware.Middleware
	logger      logger.Logger
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
	endpoint, timeout, configured := resolveConnectionConfig(target, d.httpClients, defaultEndpoint, defaultTimeout)
	if endpoint == "" {
		return nil, fmt.Errorf("http endpoint not configured for target: %s", target)
	}
	if d.logger != nil {
		helper := logger.NewHelper(d.logger)
		if configured {
			helper.Infof("using configured http endpoint: target=%s endpoint=%s", target, endpoint)
		} else {
			helper.Infof("using direct http endpoint: target=%s endpoint=%s", target, endpoint)
		}
	}

	opts := []khttp.ClientOption{
		khttp.WithEndpoint(endpoint),
		khttp.WithTimeout(timeout),
	}
	if len(d.middleware) > 0 {
		opts = append(opts, khttp.WithMiddleware(d.middleware...))
	}
	if strings.HasPrefix(endpoint, "discovery:///") && d.discovery != nil {
		opts = append(opts, khttp.WithDiscovery(d.discovery))
	}

	client, err := khttp.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("build http client for target %s: %w", target, err)
	}
	return client, nil
}

// BuildClientConfigIndex 预构建 HTTP 客户端配置索引，避免热路径重复遍历配置列表。
func BuildClientConfigIndex(dataCfg *conf.Data) (map[string]*conf.Data_Client_Endpoint, error) {
	return sharedconfig.BuildClientEndpointIndex(dataCfg, Type)
}

func resolveConnectionConfig(
	serviceName string,
	httpConfigs map[string]*conf.Data_Client_Endpoint,
	defaultEndpoint string,
	defaultTimeout time.Duration,
) (string, time.Duration, bool) {
	endpoint := defaultEndpoint
	timeout := defaultTimeout

	httpCfg, ok := httpConfigs[serviceName]
	if !ok || httpCfg == nil {
		return endpoint, timeout, false
	}

	timeout = sharedconfig.NormalizeDuration(httpCfg.GetTimeout(), defaultTimeout)
	endpoint = sharedconfig.NormalizeEndpoint(httpCfg.GetEndpoint(), defaultEndpoint)

	return endpoint, timeout, true
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

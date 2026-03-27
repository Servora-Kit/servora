package http

import (
	"context"
	"fmt"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	"github.com/Servora-Kit/servora/transport/runtime"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/registry"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type Plugin struct{}

const Type = "http"

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ClientBuildInput) (runtime.ClientFactory, error) {
	return &factory{
		httpClients: BuildClientConfigIndex(in.Data),
		discovery:   in.Discovery,
		middleware:  in.Middleware,
		logger:      in.Logger,
	}, nil
}

type factory struct {
	httpClients map[string]*conf.Data_Client_HTTP
	discovery   registry.Discovery
	middleware  []middleware.Middleware
	logger      logger.Logger
}

func (f *factory) Dial(ctx context.Context, in runtime.ClientDialInput) (runtime.Connection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	defaultTimeout := 5 * time.Second
	target := strings.TrimSpace(in.Target)
	if target == "" {
		return nil, fmt.Errorf("http dial target is empty")
	}
	endpoint, timeout, configured := resolveConnectionConfig(target, f.httpClients, target, defaultTimeout)
	if endpoint == "" {
		return nil, fmt.Errorf("http endpoint not configured for target: %s", target)
	}
	if f.logger != nil {
		helper := logger.NewHelper(f.logger)
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
	if len(f.middleware) > 0 {
		opts = append(opts, khttp.WithMiddleware(f.middleware...))
	}
	if strings.HasPrefix(endpoint, "discovery:///") && f.discovery != nil {
		opts = append(opts, khttp.WithDiscovery(f.discovery))
	}

	client, err := khttp.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("build http client for target %s: %w", target, err)
	}
	return NewSession(client, endpoint), nil
}

// BuildClientConfigIndex 预构建 HTTP 客户端配置索引，避免热路径重复遍历配置列表。
func BuildClientConfigIndex(dataCfg *conf.Data) map[string]*conf.Data_Client_HTTP {
	if dataCfg == nil || dataCfg.Client == nil {
		return nil
	}

	httpConfigs := dataCfg.Client.GetHttp()
	if len(httpConfigs) == 0 {
		return nil
	}

	index := make(map[string]*conf.Data_Client_HTTP, len(httpConfigs))
	for _, httpCfg := range httpConfigs {
		if httpCfg == nil {
			continue
		}
		serviceName := strings.TrimSpace(httpCfg.GetServiceName())
		if serviceName == "" {
			continue
		}
		index[serviceName] = httpCfg
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

func resolveConnectionConfig(
	serviceName string,
	httpConfigs map[string]*conf.Data_Client_HTTP,
	defaultEndpoint string,
	defaultTimeout time.Duration,
) (string, time.Duration, bool) {
	endpoint := defaultEndpoint
	timeout := defaultTimeout

	httpCfg, ok := httpConfigs[serviceName]
	if !ok || httpCfg == nil {
		return endpoint, timeout, false
	}

	timeout = sharedconfig.NormalizeDuration(httpCfg.GetTimeout(), timeout)
	endpoint = sharedconfig.NormalizeEndpoint(httpCfg.GetEndpoint(), endpoint)

	return endpoint, timeout, true
}

package http

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/obs/logging"
	"github.com/Servora-Kit/servora/transport/runtime"
	sharedconfig "github.com/Servora-Kit/servora/transport/shared/config"
)

type Plugin struct{}

const Type = "http"

func (p *Plugin) Type() string { return Type }

func (p *Plugin) Build(_ context.Context, in runtime.ClientBuildInput) (runtime.ClientFactory, error) {
	return &factory{
		httpClients: BuildClientConfigIndex(in.Data),
		logger:      in.Logger,
	}, nil
}

type factory struct {
	httpClients map[string]*conf.Data_Client_HTTP
	logger      logger.Logger
}

func (f *factory) CreateConn(_ context.Context, serviceName string) (runtime.Connection, error) {
	defaultTimeout := 5 * time.Second
	endpoint, timeout, configured := resolveConnectionConfig(serviceName, f.httpClients, "", defaultTimeout)
	if !configured || endpoint == "" {
		return nil, fmt.Errorf("http endpoint not configured for service: %s", serviceName)
	}
	if f.logger != nil {
		helper := logger.NewHelper(f.logger)
		helper.Infof("using configured http endpoint: service_name=%s endpoint=%s", serviceName, endpoint)
	}

	return NewConnection(&stdhttp.Client{Timeout: timeout}, endpoint), nil
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

package config

import (
	"fmt"
	"strings"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

// BuildClientEndpointIndex 按协议类型构建 service -> endpoint 配置索引。
func BuildClientEndpointIndex(dataCfg *conf.Data, protocol string) (map[string]*conf.Data_Client_Endpoint, error) {
	if dataCfg == nil || dataCfg.Client == nil {
		return nil, nil
	}

	protocol = strings.TrimSpace(strings.ToLower(protocol))
	if protocol == "" {
		return nil, fmt.Errorf("client endpoint protocol is empty")
	}

	services := dataCfg.Client.GetServices()
	if len(services) == 0 {
		return nil, nil
	}

	index := make(map[string]*conf.Data_Client_Endpoint, len(services))
	for serviceIdx, serviceCfg := range services {
		if serviceCfg == nil {
			continue
		}
		serviceName := strings.TrimSpace(serviceCfg.GetName())
		if serviceName == "" {
			return nil, fmt.Errorf("client.services[%d].name is empty", serviceIdx)
		}
		for endpointIdx, endpointCfg := range serviceCfg.GetEndpoints() {
			if endpointCfg == nil {
				continue
			}
			endpointProtocol := strings.ToLower(strings.TrimSpace(endpointCfg.GetProtocol()))
			if endpointProtocol == "" {
				return nil, fmt.Errorf("client.services[%d].endpoints[%d].protocol is empty", serviceIdx, endpointIdx)
			}
			if endpointProtocol != protocol {
				continue
			}
			if _, exists := index[serviceName]; exists {
				return nil, fmt.Errorf("duplicate client endpoint config for service=%q protocol=%q", serviceName, protocol)
			}
			index[serviceName] = endpointCfg
		}
	}

	if len(index) == 0 {
		return nil, nil
	}
	return index, nil
}

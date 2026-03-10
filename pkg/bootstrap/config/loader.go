package config

import (
	"strings"

	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	governanceConfig "github.com/Servora-Kit/servora/pkg/governance/config"

	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
)

func LoadBootstrap(configPath string, serviceName string) (*conf.Bootstrap, kconfig.Config, error) {
	envPrefix := strings.ToUpper(strings.TrimSuffix(serviceName, ".service")) + "_"
	initialSources := []kconfig.Source{file.NewSource(configPath)}

	tempConfig := kconfig.New(
		kconfig.WithSource(initialSources...),
		kconfig.WithResolveActualTypes(true),
	)
	if err := tempConfig.Load(); err != nil {
		return nil, nil, err
	}

	var bc conf.Bootstrap
	if err := tempConfig.Scan(&bc); err != nil {
		tempConfig.Close()
		return nil, nil, err
	}

	var configCenterSource kconfig.Source
	if cfg := bc.Config; cfg != nil {
		switch v := cfg.Config.(type) {
		case *conf.Config_Nacos:
			configCenterSource = governanceConfig.NewNacosConfigSource(v.Nacos)
		case *conf.Config_Consul:
			configCenterSource = governanceConfig.NewConsulConfigSource(v.Consul)
		case *conf.Config_Etcd:
			configCenterSource = governanceConfig.NewEtcdConfigSource(v.Etcd)
		}
	}
	tempConfig.Close()

	finalSources := []kconfig.Source{file.NewSource(configPath)}
	if configCenterSource != nil {
		finalSources = append(finalSources, configCenterSource)
	}
	if envPrefix != "" {
		finalSources = append(finalSources, env.NewSource(envPrefix))
	}

	c := kconfig.New(
		kconfig.WithSource(finalSources...),
		kconfig.WithResolveActualTypes(true),
	)
	if err := c.Load(); err != nil {
		return nil, nil, err
	}
	if err := c.Scan(&bc); err != nil {
		c.Close()
		return nil, nil, err
	}

	return &bc, c, nil
}

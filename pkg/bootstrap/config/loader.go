package config

import (
	"strings"

	conf "github.com/horonlee/servora/api/gen/go/conf/v1"
	governanceConfig "github.com/horonlee/servora/pkg/governance/config"

	krconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
)

func LoadBootstrap(configPath string, serviceName string) (*conf.Bootstrap, krconfig.Config, error) {
	envPrefix := strings.ToUpper(strings.TrimSuffix(serviceName, ".service")) + "_"
	initialSources := []krconfig.Source{file.NewSource(configPath)}

	tempConfig := krconfig.New(
		krconfig.WithSource(initialSources...),
		krconfig.WithResolveActualTypes(true),
	)
	if err := tempConfig.Load(); err != nil {
		return nil, nil, err
	}

	var bc conf.Bootstrap
	if err := tempConfig.Scan(&bc); err != nil {
		tempConfig.Close()
		return nil, nil, err
	}

	var configCenterSource krconfig.Source
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

	finalSources := []krconfig.Source{file.NewSource(configPath)}
	if configCenterSource != nil {
		finalSources = append(finalSources, configCenterSource)
	}
	if envPrefix != "" {
		finalSources = append(finalSources, env.NewSource(envPrefix))
	}

	c := krconfig.New(
		krconfig.WithSource(finalSources...),
		krconfig.WithResolveActualTypes(true),
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

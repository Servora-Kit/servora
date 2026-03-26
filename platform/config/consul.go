package config

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/contrib/config/consul/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/hashicorp/consul/api"
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

// NewConsulConfigSource 创建 Consul 配置源
func NewConsulConfigSource(c *conf.ConsulConfig) config.Source {
	if c == nil {
		return nil
	}

	// 创建 Consul 客户端配置
	consulConfig := api.DefaultConfig()
	consulConfig.Address = c.Addr
	consulConfig.Scheme = c.Scheme
	consulConfig.Token = c.Token
	consulConfig.Datacenter = c.Datacenter

	// 设置超时时间
	if c.Timeout != nil {
		consulConfig.WaitTime = c.Timeout.AsDuration()
	} else {
		consulConfig.WaitTime = 5 * time.Second
	}

	// 创建 Consul 客户端
	client, err := api.NewClient(consulConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to create consul client: %v", err))
	}

	// 设置配置键名，默认为 config
	key := "config"
	if c.Key != "" {
		key = c.Key
	}

	source, err := consul.New(client, consul.WithPath(key))
	if err != nil {
		panic(fmt.Sprintf("failed to create consul config source: %v", err))
	}

	return source
}

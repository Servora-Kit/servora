package registry

import (
	"fmt"

	"github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/go-kratos/kratos/v2/registry"
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// NewNacosRegistry 创建 Nacos 统一注册中心客户端（支持注册和发现）
func NewNacosRegistry(c *conf.NacosConfig) registry.Registrar {
	if c == nil {
		return nil
	}

	// 创建 Nacos 服务端配置
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(c.Addr, c.Port),
	}

	timeoutMs := uint64(5000)
	if c.Timeout != nil && c.Timeout.AsDuration() > 0 {
		timeoutMs = uint64(c.Timeout.AsDuration().Milliseconds())
	}

	// 创建 Nacos 客户端配置
	cc := &constant.ClientConfig{
		NamespaceId:         c.Namespace,
		Username:            c.Username,
		Password:            c.Password,
		TimeoutMs:           timeoutMs,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
	}

	// 创建命名客户端
	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create nacos client: %v", err))
	}

	// 创建group参数，如果未设置则使用默认值
	group := c.Group
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	// 创建 Nacos 注册中心
	r := nacos.New(client, nacos.WithGroup(group))
	return r
}

// NewNacosDiscovery 创建 Nacos 服务发现客户端
func NewNacosDiscovery(c *conf.NacosConfig) registry.Discovery {
	if c == nil {
		return nil
	}

	// 创建 Nacos 服务端配置
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(c.Addr, c.Port),
	}

	timeoutMs := uint64(5000)
	if c.Timeout != nil && c.Timeout.AsDuration() > 0 {
		timeoutMs = uint64(c.Timeout.AsDuration().Milliseconds())
	}

	// 创建 Nacos 客户端配置
	cc := &constant.ClientConfig{
		NamespaceId:         c.Namespace,
		Username:            c.Username,
		Password:            c.Password,
		TimeoutMs:           timeoutMs,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
	}

	// 创建命名客户端
	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create nacos client: %v", err))
	}

	// 创建group参数，如果未设置则使用默认值
	group := c.Group
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	// 创建 Nacos 服务发现
	r := nacos.New(client, nacos.WithGroup(group))
	return r
}

// NewNacosRegistrar 创建 Nacos 注册中心客户端
// Deprecated: 使用 NewNacosRegistry 替代
func NewNacosRegistrar(c *conf.NacosConfig) registry.Registrar {
	return NewNacosRegistry(c)
}

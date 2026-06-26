package registry

import (
	"fmt"
	"time"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"

	"github.com/go-kratos/kratos/v3/registry"
)

func NewRegistrar(cfg *corev1.Registry) registry.Registrar {
	if cfg == nil {
		return nil
	}

	switch c := cfg.Registry.(type) {
	case *corev1.Registry_Consul:
		return NewConsulRegistry(c.Consul)
	case *corev1.Registry_Etcd:
		var opts []Option
		if c.Etcd.Namespace != "" {
			opts = append(opts, Namespace(c.Etcd.Namespace))
		}
		opts = append(opts, RegisterTTL(15*time.Second), MaxRetry(5))
		registrar, err := NewEtcdRegistry(c.Etcd, opts...)
		if err != nil {
			panic(fmt.Sprintf("failed to create etcd registry: %v", err))
		}
		return registrar
	case *corev1.Registry_Nacos:
		return NewNacosRegistry(c.Nacos)
	case *corev1.Registry_Kubernetes:
		return NewKubernetesRegistry(c.Kubernetes)
	default:
		return nil
	}
}

// NewDiscovery 根据注册中心配置创建服务发现客户端。
func NewDiscovery(cfg *corev1.Registry) registry.Discovery {
	if cfg == nil {
		return nil
	}

	switch c := cfg.Registry.(type) {
	case *corev1.Registry_Consul:
		return NewConsulDiscovery(c.Consul)
	case *corev1.Registry_Etcd:
		var opts []Option
		if c.Etcd.Namespace != "" {
			opts = append(opts, Namespace(c.Etcd.Namespace))
		}
		opts = append(opts, RegisterTTL(15*time.Second), MaxRetry(5))
		discovery, err := NewEtcdDiscovery(c.Etcd, opts...)
		if err != nil {
			panic(fmt.Sprintf("failed to create etcd discovery: %v", err))
		}
		return discovery
	case *corev1.Registry_Nacos:
		return NewNacosDiscovery(c.Nacos)
	case *corev1.Registry_Kubernetes:
		return NewKubernetesDiscovery(c.Kubernetes)
	default:
		return nil
	}
}

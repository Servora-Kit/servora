package registry

import (
	"fmt"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/Servora-Kit/servora/infra/k8s"
	kuberegistry "github.com/go-kratos/kratos/contrib/registry/kubernetes/v2"
	"github.com/go-kratos/kratos/v2/registry"
)

func NewKubernetesRegistry(c *corev1.Kubernetes) registry.Registrar {
	if c == nil || !c.Enable {
		return nil
	}

	clientset, err := k8s.NewClientset()
	if err != nil {
		panic(fmt.Sprintf("failed to create kubernetes clientset: %v", err))
	}

	reg := kuberegistry.NewRegistry(clientset, k8s.GetCurrentNamespace())
	reg.Start()
	return reg
}

func NewKubernetesDiscovery(c *corev1.Kubernetes) registry.Discovery {
	if c == nil || !c.Enable {
		return nil
	}

	clientset, err := k8s.NewClientset()
	if err != nil {
		panic(fmt.Sprintf("failed to create kubernetes clientset: %v", err))
	}

	reg := kuberegistry.NewRegistry(clientset, k8s.GetCurrentNamespace())
	reg.Start()
	return reg
}

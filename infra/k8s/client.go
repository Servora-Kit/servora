package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClientset 创建 Kubernetes Clientset
// 优先使用 InClusterConfig，失败则回退到 kubeconfig
func NewClientset() (*kubernetes.Clientset, error) {
	config, err := buildConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return cs, nil
}

func buildConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = clientcmd.RecommendedHomeFile
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// GetCurrentNamespace 获取当前 Pod 所在的命名空间
func GetCurrentNamespace() string {
	ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil && len(ns) > 0 {
		return string(ns)
	}

	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	return "default"
}

// GetPodName 获取当前 Pod 名称
func GetPodName() string {
	if name := os.Getenv("HOSTNAME"); name != "" {
		return name
	}
	hostname, _ := os.Hostname()
	return hostname
}

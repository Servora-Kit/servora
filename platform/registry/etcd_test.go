package registry

import (
	"context"
	"os"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

func shouldRunEtcdIntegration() bool {
	return os.Getenv("RUN_ETCD_INTEGRATION_TESTS") == "1"
}

// TestEtcdRegistryAndDiscovery 测试 etcd 注册和发现功能
func TestEtcdRegistryAndDiscovery(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	// 创建 etcd 客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Skipf("Skipping test: could not connect to etcd: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	// 创建注册中心实例
	reg := New(client,
		Namespace("/test-servora"),
		RegisterTTL(10*time.Second),
		MaxRetry(3),
	)

	// 测试服务实例
	serviceInstance := &registry.ServiceInstance{
		ID:      "test-service-1",
		Name:    "test-service",
		Version: "v1.0.0",
		Metadata: map[string]string{
			"region": "local",
			"zone":   "zone1",
		},
		Endpoints: []string{
			"grpc://127.0.0.1:9001?isSecure=false",
		},
	}

	// 测试服务注册
	ctx := context.Background()
	err = reg.Register(ctx, serviceInstance)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}
	t.Logf("Service registered successfully")

	// 等待注册生效
	time.Sleep(1 * time.Second)

	// 测试服务发现
	services, err := reg.GetService(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if len(services) == 0 {
		t.Fatalf("No services found")
	}

	found := false
	for _, svc := range services {
		if svc.ID == serviceInstance.ID {
			found = true
			if svc.Name != serviceInstance.Name {
				t.Errorf("Expected name %s, got %s", serviceInstance.Name, svc.Name)
			}
			if svc.Version != serviceInstance.Version {
				t.Errorf("Expected version %s, got %s", serviceInstance.Version, svc.Version)
			}
			break
		}
	}

	if !found {
		t.Fatalf("Service instance not found in discovery")
	}

	t.Logf("Service discovered successfully: %+v", services[0])

	// 测试服务监听
	watcher, err := reg.Watch(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer func() {
		if err := watcher.Stop(); err != nil {
			t.Logf("watcher stop failed: %v", err)
		}
	}()

	// 测试注销服务
	err = reg.Deregister(ctx, serviceInstance)
	if err != nil {
		t.Fatalf("Failed to deregister service: %v", err)
	}
	t.Logf("Service deregistered successfully")

	// 等待注销生效
	time.Sleep(1 * time.Second)

	// 验证服务已被移除
	services, err = reg.GetService(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to get service after deregister: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services after deregister, got %d", len(services))
	}
}

// TestEtcdConfigConversion 测试配置转换
func TestEtcdConfigConversion(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	// 创建测试配置
	cfg := &conf.EtcdConfig{
		Endpoints: []string{"127.0.0.1:2379", "127.0.0.1:2380"},
		Username:  "testuser",
		Password:  "testpass",
	}

	// 测试注册中心创建
	registrar, err := NewEtcdRegistry(cfg,
		Namespace("/test-servora"),
		RegisterTTL(15*time.Second),
		MaxRetry(5),
	)
	if err != nil {
		// 如果没有 etcd 服务器，跳过测试
		t.Skipf("Skipping test: could not create etcd registry: %v", err)
		return
	}

	if registrar == nil {
		t.Error("Expected registrar, got nil")
	}

	// 测试服务发现创建
	discovery, err := NewEtcdDiscovery(cfg,
		Namespace("/test-servora"),
		RegisterTTL(15*time.Second),
		MaxRetry(5),
	)
	if err != nil {
		t.Skipf("Skipping test: could not create etcd discovery: %v", err)
		return
	}

	if discovery == nil {
		t.Error("Expected discovery, got nil")
	}

	t.Logf("Configuration conversion test passed")
}

// BenchmarkEtcdServiceDiscovery 性能基准测试
func BenchmarkEtcdServiceDiscovery(b *testing.B) {
	if !shouldRunEtcdIntegration() {
		b.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration benchmark")
	}

	// 创建 etcd 客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		b.Skipf("Skipping benchmark: could not connect to etcd: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	reg := New(client, Namespace("/bench-servora"))

	// 注册测试服务
	serviceInstance := &registry.ServiceInstance{
		ID:      "bench-service-1",
		Name:    "bench-service",
		Version: "v1.0.0",
		Endpoints: []string{
			"grpc://127.0.0.1:9001?isSecure=false",
		},
	}

	ctx := context.Background()
	err = reg.Register(ctx, serviceInstance)
	if err != nil {
		b.Skipf("Skipping benchmark: could not register service: %v", err)
		return
	}
	defer func() {
		if err := reg.Deregister(ctx, serviceInstance); err != nil {
			b.Logf("cleanup deregister failed: %v", err)
		}
	}()

	// 等待注册生效
	time.Sleep(1 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reg.GetService(ctx, "bench-service")
		if err != nil {
			b.Errorf("Failed to get service: %v", err)
		}
	}
}

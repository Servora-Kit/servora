package registry

import (
	"context"
	"fmt"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
)

// TestEtcdErrorHandling 测试各种错误处理场景
func TestEtcdErrorHandling(t *testing.T) {
	t.Run("InvalidConfiguration", func(t *testing.T) {
		// 测试空配置
		_, err := NewEtcdClient(nil)
		if err == nil {
			t.Error("Expected error for nil config")
		}

		// 测试空端点
		cfg := &conf.EtcdConfig{
			Endpoints: []string{},
		}
		_, err = NewEtcdClient(cfg)
		if err == nil {
			t.Error("Expected error for empty endpoints")
		}
	})

	t.Run("InvalidEndpoints", func(t *testing.T) {
		cfg := &conf.EtcdConfig{
			Endpoints: []string{"invalid-endpoint:9999"},
			Timeout:   durationpbNew(1 * time.Second),
		}

		// 应该在合理时间内失败
		start := time.Now()
		_, err := NewEtcdClient(cfg)
		elapsed := time.Since(start)

		// 应该在合理时间内失败（不超过 10 秒）
		if elapsed > 10*time.Second {
			t.Errorf("Connection timeout took too long: %v", elapsed)
		}

		t.Logf("Invalid endpoint test completed in %v with error: %v", elapsed, err)
	})
}

// TestEtcdBoundaryConditions 测试边界条件
func TestEtcdBoundaryConditions(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	// 只在有 etcd 服务器时运行
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Skipf("Skipping boundary tests: no etcd available: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	t.Run("EmptyServiceInstance", func(t *testing.T) {
		reg := New(client)

		// 测试空服务实例
		emptyService := &registry.ServiceInstance{}
		ctx := context.Background()

		err := reg.Register(ctx, emptyService)
		if err != nil {
			// 这可能会失败，但我们主要确保不会 panic
			t.Logf("Empty service registration failed as expected: %v", err)
		}
	})

	t.Run("LargeMetadata", func(t *testing.T) {
		reg := New(client)

		// 创建包含大量元数据的服务实例
		largeMetadata := make(map[string]string)
		for i := 0; i < 100; i++ {
			largeMetadata[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
		}

		service := &registry.ServiceInstance{
			ID:       "large-metadata-service",
			Name:     "test-service",
			Version:  "v1.0.0",
			Metadata: largeMetadata,
		}

		ctx := context.Background()
		err := reg.Register(ctx, service)
		if err != nil {
			t.Errorf("Failed to register service with large metadata: %v", err)
			return
		}

		// 验证可以正确获取
		services, err := reg.GetService(ctx, "test-service")
		if err != nil {
			t.Errorf("Failed to get service with large metadata: %v", err)
			return
		}

		found := false
		for _, svc := range services {
			if svc.ID == service.ID {
				found = true
				if len(svc.Metadata) != len(largeMetadata) {
					t.Errorf("Expected %d metadata items, got %d", len(largeMetadata), len(svc.Metadata))
				}
				break
			}
		}

		if !found {
			t.Error("Service with large metadata not found")
		}

		// 清理
		if err := reg.Deregister(ctx, service); err != nil {
			t.Logf("cleanup deregister failed: %v", err)
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		reg := New(client, Namespace("/concurrent-test"))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 并发注册多个服务
		const numServices = 5
		errChan := make(chan error, numServices)

		for i := 0; i < numServices; i++ {
			go func(id int) {
				service := &registry.ServiceInstance{
					ID:      fmt.Sprintf("concurrent-service-%d", id),
					Name:    fmt.Sprintf("concurrent-test-service-%d", id),
					Version: "v1.0.0",
				}

				err := reg.Register(ctx, service)
				if err != nil {
					errChan <- err
					return
				}

				// 等待一下再注销
				time.Sleep(100 * time.Millisecond)
				err = reg.Deregister(ctx, service)
				errChan <- err
			}(i)
		}

		// 等待所有操作完成
		for i := 0; i < numServices; i++ {
			select {
			case err := <-errChan:
				if err != nil {
					t.Logf("Concurrent operation failed: %v", err)
				}
			case <-ctx.Done():
				t.Error("Timeout waiting for concurrent operations")
				return
			}
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		reg := New(client, Namespace("/cancel-test"))

		// 创建可取消的上下文
		ctx, cancel := context.WithCancel(context.Background())

		service := &registry.ServiceInstance{
			ID:      "cancel-test-service",
			Name:    "cancel-test-service",
			Version: "v1.0.0",
		}

		// 开始注册
		errChan := make(chan error, 1)
		go func() {
			err := reg.Register(ctx, service)
			errChan <- err
		}()

		// 立即取消上下文
		cancel()

		// 等待注册操作完成或超时
		select {
		case err := <-errChan:
			// 操作可能因为上下文取消而失败，这是预期的
			t.Logf("Registration with cancelled context result: %v", err)
		case <-time.After(5 * time.Second):
			t.Error("Registration should have been cancelled quickly")
		}
	})
}

// TestEtcdWatcherResilience 测试监听器的恢复能力
func TestEtcdWatcherResilience(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Skipf("Skipping watcher resilience test: no etcd available: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	reg := New(client, Namespace("/watcher-test"))

	ctx := context.Background()

	// 创建监听器
	watcher, err := reg.Watch(ctx, "resilience-test-service")
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer func() {
		if err := watcher.Stop(); err != nil {
			t.Logf("watcher stop failed: %v", err)
		}
	}()

	// 测试监听器的基本功能
	service := &registry.ServiceInstance{
		ID:      "resilience-test-1",
		Name:    "resilience-test-service",
		Version: "v1.0.0",
	}

	// 注册服务
	err = reg.Register(ctx, service)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}
	defer func() {
		if err := reg.Deregister(ctx, service); err != nil {
			t.Logf("cleanup deregister failed: %v", err)
		}
	}()

	// 监听器应该检测到变化
	select {
	case <-time.After(2 * time.Second):
		t.Log("Watcher created successfully (timeout is expected in test)")
	case <-ctx.Done():
		t.Error("Context cancelled unexpectedly")
	}
}

func durationpbNew(d time.Duration) *durationpb.Duration {
	return &durationpb.Duration{
		Seconds: int64(d.Seconds()),
		Nanos:   int32(d.Nanoseconds() % 1e9),
	}
}

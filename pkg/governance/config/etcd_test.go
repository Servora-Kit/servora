package config

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const testKey = "/kratos/test/config"

func shouldRunEtcdIntegration() bool {
	return os.Getenv("RUN_ETCD_INTEGRATION_TESTS") == "1"
}

func TestConfig(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second,
	})
	if err != nil {
		t.Skipf("etcd not available: %v", err)
		return
	}
	defer func() {
		_ = client.Close()
	}()

	// 清理测试环境
	_, _ = client.Delete(context.Background(), testKey)

	if _, err = client.Put(context.Background(), testKey, "test config"); err != nil {
		t.Fatal(err)
	}

	source, err := New(client, WithPath(testKey))
	if err != nil {
		t.Fatal(err)
	}

	kvs, err := source.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(kvs) != 1 || kvs[0].Key != testKey || string(kvs[0].Value) != "test config" {
		t.Fatal("config error")
	}

	w, err := source.Watch()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = w.Stop()
	}()

	if _, err = client.Put(context.Background(), testKey, "new config"); err != nil {
		t.Error(err)
	}

	if kvs, err = w.Next(); err != nil {
		t.Fatal(err)
	}

	if len(kvs) != 1 || kvs[0].Key != testKey || string(kvs[0].Value) != "new config" {
		t.Fatal("config error")
	}

	if _, err := client.Delete(context.Background(), testKey); err != nil {
		t.Error(err)
	}
}

func TestExtToFormat(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second,
	})
	if err != nil {
		t.Skipf("etcd not available: %v", err)
		return
	}
	defer func() {
		_ = client.Close()
	}()

	tp := "/kratos/test/ext"
	tn := "a.bird.json"
	tk := tp + "/" + tn
	tc := `{"a":1}`

	// 清理测试环境
	_, _ = client.Delete(context.Background(), tk)

	if _, err = client.Put(context.Background(), tk, tc); err != nil {
		t.Fatal(err)
	}

	source, err := New(client, WithPath(tp), WithPrefix(true))
	if err != nil {
		t.Fatal(err)
	}

	kvs, err := source.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(len(kvs), 1) {
		t.Errorf("len(kvs) = %d", len(kvs))
	}
	if !reflect.DeepEqual(tk, kvs[0].Key) {
		t.Errorf("kvs[0].Key is %s", kvs[0].Key)
	}
	if !reflect.DeepEqual(tc, string(kvs[0].Value)) {
		t.Errorf("kvs[0].Value is %s", kvs[0].Value)
	}
	if !reflect.DeepEqual("json", kvs[0].Format) {
		t.Errorf("kvs[0].Format is %s", kvs[0].Format)
	}
}

func TestEtcdWithPath(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		want   string
	}{
		{
			name:   "default",
			fields: testKey,
			want:   testKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &options{
				ctx: context.Background(),
			}

			got := WithPath(tt.fields)
			got(options)

			if options.path != tt.want {
				t.Errorf("WithPath(tt.fields) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEtcdWithPrefix(t *testing.T) {
	tests := []struct {
		name   string
		fields bool
		want   bool
	}{
		{
			name:   "default",
			fields: false,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &options{
				ctx: context.Background(),
			}

			got := WithPrefix(tt.fields)
			got(options)

			if options.prefix != tt.want {
				t.Errorf("WithPrefix(tt.fields) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewEtcdConfigSource(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	config := &conf.EtcdConfig{
		Endpoints: []string{"127.0.0.1:2379"},
		Key:       "/test/config",
	}

	source := NewEtcdConfigSource(config)
	if source == nil {
		t.Fatal("source is nil")
	}

	// 测试加载配置
	kvs, err := source.Load()
	if err != nil {
		t.Errorf("Load() error = %v", err)
	}

	// 清理测试环境
	client, _ := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second,
	})
	if client != nil {
		_, _ = client.Delete(context.Background(), "/test/config")
		_ = client.Close()
	}

	t.Logf("loaded %d config items", len(kvs))
}

func TestSourceNilConfig(t *testing.T) {
	source := NewEtcdConfigSource(nil)
	if source != nil {
		t.Errorf("NewEtcdConfigSource(nil) = %v, want nil", source)
	}
}

func TestWatcher(t *testing.T) {
	if !shouldRunEtcdIntegration() {
		t.Skip("set RUN_ETCD_INTEGRATION_TESTS=1 to run etcd integration tests")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second,
	})
	if err != nil {
		t.Skipf("etcd not available: %v", err)
		return
	}
	defer func() {
		_ = client.Close()
	}()

	testPath := "/kratos/test/watcher"
	// 清理测试环境
	_, _ = client.Delete(context.Background(), testPath, clientv3.WithPrefix())

	source, err := New(client, WithPath(testPath), WithPrefix(true))
	if err != nil {
		t.Fatal(err)
	}

	watcher, err := source.Watch()
	if err != nil {
		t.Fatal(err)
	}

	// 测试停止监听器
	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// 测试监听配置变化
	watcher2, err := source.Watch()
	if err != nil {
		t.Fatal(err)
	}

	// 在另一个 goroutine 中更新配置
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = client.Put(context.Background(), testPath+"/test1", "value1")
	}()

	// 等待配置更新
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error)
	go func() {
		_, err := watcher2.Next()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Next() error = %v", err)
		}
	case <-ctx.Done():
		t.Error("Next() timeout")
	}

	_ = watcher2.Stop()
}

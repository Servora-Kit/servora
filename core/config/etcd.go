package config

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Option is etcd config option.
type Option func(o *options)

type options struct {
	ctx    context.Context
	path   string
	prefix bool
}

// WithContext with registry context.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.ctx = ctx
	}
}

// WithPath is config path
func WithPath(p string) Option {
	return func(o *options) {
		o.path = p
	}
}

// WithPrefix is config prefix
func WithPrefix(prefix bool) Option {
	return func(o *options) {
		o.prefix = prefix
	}
}

// source 实现 config.Source 接口的 Etcd 配置源
type source struct {
	client  *clientv3.Client
	options *options
}

// NewEtcdConfigSource 创建 Etcd 配置源（兼容旧版本）
func NewEtcdConfigSource(c *conf.EtcdConfig) config.Source {
	if c == nil {
		return nil
	}

	etcdConfig := clientv3.Config{
		Endpoints: c.Endpoints,
		Username:  c.Username,
		Password:  c.Password,
	}

	if c.Timeout != nil {
		etcdConfig.DialTimeout = c.Timeout.AsDuration()
	} else {
		etcdConfig.DialTimeout = 5 * time.Second
	}

	client, err := clientv3.New(etcdConfig)
	if err != nil {
		panic("failed to create etcd client: " + err.Error())
	}

	path := "/config"
	if c.Key != "" {
		path = c.Key
	} else if c.Namespace != "" {
		path = c.Namespace + "/config.yaml"
	}

	return &source{
		client: client,
		options: &options{
			ctx:    context.Background(),
			path:   path,
			prefix: false,
		},
	}
}

// New 创建 Etcd 配置源（官方标准接口）
func New(client *clientv3.Client, opts ...Option) (config.Source, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}

	options := &options{
		ctx:    context.Background(),
		path:   "",
		prefix: false,
	}

	for _, opt := range opts {
		opt(options)
	}

	if options.path == "" {
		return nil, errors.New("path invalid")
	}

	return &source{
		client:  client,
		options: options,
	}, nil
}

// Load 实现 config.Source 接口
func (s *source) Load() ([]*config.KeyValue, error) {
	var opts []clientv3.OpOption
	if s.options.prefix {
		opts = append(opts, clientv3.WithPrefix())
	}

	rsp, err := s.client.Get(s.options.ctx, s.options.path, opts...)
	if err != nil {
		return nil, err
	}

	kvs := make([]*config.KeyValue, 0, len(rsp.Kvs))
	for _, item := range rsp.Kvs {
		k := string(item.Key)
		kvs = append(kvs, &config.KeyValue{
			Key:    k,
			Value:  item.Value,
			Format: strings.TrimPrefix(filepath.Ext(k), "."),
		})
	}
	return kvs, nil
}

// Watch 实现 config.Source 接口
func (s *source) Watch() (config.Watcher, error) {
	return newWatcher(s), nil
}

// Close 关闭配置源
func (s *source) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// watcher 实现 config.Watcher 接口的 Etcd 监听器
type watcher struct {
	source *source
	ch     clientv3.WatchChan

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

func newWatcher(s *source) *watcher {
	ctx, cancel := context.WithCancel(context.Background())
	w := &watcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
	}

	var opts []clientv3.OpOption
	if s.options.prefix {
		opts = append(opts, clientv3.WithPrefix())
	}
	w.ch = s.client.Watch(s.options.ctx, s.options.path, opts...)

	return w
}

// Next 实现 config.Watcher 接口
func (w *watcher) Next() ([]*config.KeyValue, error) {
	select {
	case resp := <-w.ch:
		if err := resp.Err(); err != nil {
			return nil, err
		}
		// 返回所有当前配置，而不仅仅是变更的部分
		return w.source.Load()
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	}
}

// Stop 实现 config.Watcher 接口
func (w *watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}
	return nil
}

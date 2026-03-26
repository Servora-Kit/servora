package server

import (
	"context"
	"net/url"
)

// Lifecycle 定义服务启停契约。
type Lifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// EndpointProvider 定义服务注册地址契约。
type EndpointProvider interface {
	Endpoint() (*url.URL, error)
}

// Server 聚合 transport 扩展服务的最小能力集合。
type Server interface {
	Lifecycle
	EndpointProvider
}

package client

import (
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/go-kratos/kratos/v2/middleware"
)

type Option func(*options)

type options struct {
	registerBuiltin bool
	plugins         []runtime.ClientPlugin
	registry        *runtime.Registry
	middleware      []middleware.Middleware
}

func defaultOptions() options {
	return options{
		registerBuiltin: true,
	}
}

// WithPlugins 追加外部 client plugins。
func WithPlugins(plugins ...runtime.ClientPlugin) Option {
	return func(o *options) {
		o.plugins = append(o.plugins, plugins...)
	}
}

// WithoutBuiltinPlugins 关闭内建 client plugins 自动注册。
func WithoutBuiltinPlugins() Option {
	return func(o *options) {
		o.registerBuiltin = false
	}
}

// WithRegistry 注入已有 runtime registry。
func WithRegistry(r *runtime.Registry) Option {
	return func(o *options) {
		o.registry = r
	}
}

// WithMiddleware 追加 client 治理中间件。
func WithMiddleware(mw ...middleware.Middleware) Option {
	return func(o *options) {
		o.middleware = append(o.middleware, mw...)
	}
}

package grpc

import (
	"log/slog"

	corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
	"github.com/go-kratos/kratos/v3/middleware"
	kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
)

type Registrar func(*kgrpc.Server)

type ServerOption func(*serverOptions)

type serverOptions struct {
	conf       *corev1.Server_GRPC
	logger     *slog.Logger
	middleware []middleware.Middleware
	registrars []Registrar
}

func WithConfig(c *corev1.Server_GRPC) ServerOption {
	return func(o *serverOptions) {
		o.conf = c
	}
}

func WithLogger(l *slog.Logger) ServerOption {
	return func(o *serverOptions) {
		o.logger = l
	}
}

func WithMiddleware(mw ...middleware.Middleware) ServerOption {
	return func(o *serverOptions) {
		o.middleware = mw
	}
}

func WithServices(registrars ...Registrar) ServerOption {
	return func(o *serverOptions) {
		o.registrars = registrars
	}
}

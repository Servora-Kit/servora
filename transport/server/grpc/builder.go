package grpc

import (
	"context"
	"fmt"

	conf "github.com/Servora-Kit/servora/api/gen/go/servora/conf/v1"
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

// Builder 提供面向调用方的 gRPC server DSL，隐藏 runtime graph 细节。
type Builder struct {
	config     *conf.Server_GRPC
	logger     log.Logger
	middleware []middleware.Middleware
	registrars []Registrar
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithConfig(c *conf.Server_GRPC) *Builder {
	b.config = c
	return b
}

func (b *Builder) WithLogger(l log.Logger) *Builder {
	b.logger = l
	return b
}

func (b *Builder) WithMiddleware(mw ...middleware.Middleware) *Builder {
	b.middleware = append(b.middleware, mw...)
	return b
}

func (b *Builder) WithServices(registrars ...Registrar) *Builder {
	b.registrars = append(b.registrars, registrars...)
	return b
}

func (b *Builder) Build(ctx context.Context) (*kgrpc.Server, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	in := runtime.ServerBuildInput{
		Config:     b.config,
		Logger:     b.logger,
		Middleware: b.middleware,
	}
	if len(b.registrars) > 0 {
		in.Registrars = make([]any, 0, len(b.registrars))
		for _, reg := range b.registrars {
			if reg == nil {
				continue
			}
			in.Registrars = append(in.Registrars, reg)
		}
	}

	raw, err := (&Plugin{}).Build(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("build grpc server from plugin: %w", err)
	}

	srv, ok := raw.(*kgrpc.Server)
	if !ok {
		return nil, fmt.Errorf("unexpected grpc server type: %T", raw)
	}

	return srv, nil
}

func (b *Builder) MustBuild() *kgrpc.Server {
	srv, err := b.Build(context.Background())
	if err != nil {
		panic(err)
	}
	return srv
}

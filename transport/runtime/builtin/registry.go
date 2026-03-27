package builtin

import (
	"fmt"

	"github.com/Servora-Kit/servora/transport/client"
	clientgrpc "github.com/Servora-Kit/servora/transport/client/grpc"
	clienthttp "github.com/Servora-Kit/servora/transport/client/http"
	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/Servora-Kit/servora/transport/server"
	grpcserver "github.com/Servora-Kit/servora/transport/server/grpc"
	httpserver "github.com/Servora-Kit/servora/transport/server/http"
)

// NewRegistry 创建包含内建协议插件的 runtime registry。
func NewRegistry() (*runtime.Registry, error) {
	r := runtime.NewRegistry()
	if err := RegisterAll(r); err != nil {
		return nil, err
	}
	return r, nil
}

// RegisterAll 注册框架内建 transport plugins。
func RegisterAll(r *runtime.Registry) error {
	if r == nil {
		return fmt.Errorf("runtime registry is nil")
	}

	if err := server.RegisterPlugins(r,
		&grpcserver.Plugin{},
		&httpserver.Plugin{},
	); err != nil {
		return fmt.Errorf("register builtin server plugins: %w", err)
	}

	if err := client.RegisterPlugins(r,
		&clientgrpc.Plugin{},
		&clienthttp.Plugin{},
	); err != nil {
		return fmt.Errorf("register builtin client plugins: %w", err)
	}

	return nil
}

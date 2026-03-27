package client

import (
	"context"

	"github.com/Servora-Kit/servora/transport/runtime"
	"github.com/google/wire"
)

// ProviderSet 是客户端的依赖注入提供者集合。
var ProviderSet = wire.NewSet(
	NewDefaultClient,
)

// Client 客户端接口。
type Client interface {
	Dial(ctx context.Context, in runtime.ClientDialInput) (runtime.Connection, error)
}

package client

import "github.com/google/wire"

// ProviderSet 是客户端的依赖注入提供者集合
var ProviderSet = wire.NewSet(
	NewDefaultClient,
)

// ConnType 连接类型枚举
type ConnType string

const (
	GRPC ConnType = "grpc"
	HTTP ConnType = "http"
)

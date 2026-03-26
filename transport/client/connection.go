package client

import "context"

// Connection 顶层连接接口
type Connection interface {
	// Value 返回原始连接对象
	Value() any
	// Close 关闭连接
	Close() error
	// IsHealthy 检查连接健康状态
	IsHealthy() bool
	// GetType 返回连接类型
	GetType() ConnType
}

// Client 客户端接口
type Client interface {
	// CreateConn 创建指定类型的连接
	CreateConn(ctx context.Context, connType ConnType, serviceName string) (Connection, error)
}

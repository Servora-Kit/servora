package client

import (
	"context"
	"fmt"
)

// GetConnValue 按连接类型创建连接并提取底层值。
func GetConnValue[T any](ctx context.Context, c Client, connType ConnType, serviceName string) (T, error) {
	var zero T
	connWrapper, err := c.CreateConn(ctx, connType, serviceName)
	if err != nil {
		return zero, err
	}
	conn, ok := connWrapper.Value().(T)
	if !ok {
		return zero, fmt.Errorf("unexpected %s connection type: %T", connType, connWrapper.Value())
	}
	return conn, nil
}

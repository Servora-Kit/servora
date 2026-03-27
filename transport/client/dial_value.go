package client

import (
	"context"
	"fmt"

	"github.com/Servora-Kit/servora/transport/runtime"
)

// GetValue 按 DialInput 建立连接并提取底层值。
func GetValue[T any](ctx context.Context, c Client, in runtime.ClientDialInput) (T, error) {
	var zero T
	sess, err := c.Dial(ctx, in)
	if err != nil {
		return zero, err
	}
	conn, ok := sess.Value().(T)
	if !ok {
		return zero, fmt.Errorf("unexpected %s session value type: %T", in.Protocol, sess.Value())
	}
	return conn, nil
}

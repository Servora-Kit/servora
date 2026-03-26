package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
)

// TokenPropagation 创建一个客户端中间件，用于将 bearer token 从传入的请求上下文转发到下游服务调用。
//
// 它会读取由服务端认证中间件存储在上下文中的 token，并在发出的请求中设置为 Authorization 头。
func TokenPropagation() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			token, ok := svrmw.TokenFromContext(ctx)
			if !ok || token == "" {
				// 如果上下文中没有 token，则直接调用下一个 handler
				return handler(ctx, req)
			}

			tr, ok := transport.FromClientContext(ctx)
			if !ok {
				// 如果无法从上下文获取客户端传输信息，则直接调用下一个 handler
				return handler(ctx, req)
			}

			tr.RequestHeader().Set("Authorization", "Bearer "+token)
			// 将 token 以 Authorization 头的形式传递到下游服务
			return handler(ctx, req)
		}
	}
}

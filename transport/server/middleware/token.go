package middleware

import "context"

type tokenKey struct{}

// TokenFromContext 从 context 中获取由 authn 中间件存储的原始 Bearer token，
// 用于跨服务调用时的 token 传播。
func TokenFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tokenKey{}).(string)
	return t, ok
}

// NewTokenContext 将原始 Bearer token 存储到 context 中。
func NewTokenContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}

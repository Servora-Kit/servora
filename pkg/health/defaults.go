package health

import "database/sql"

// DefaultDeps 定义默认健康检查的依赖项。
type DefaultDeps struct {
	Redis Pinger
	DB    *sql.DB
}

// NewHandlerWithDefaults 使用默认依赖创建 Handler。
// 自动注册所有非 nil 的依赖项为 checker。
func NewHandlerWithDefaults(deps DefaultDeps) *Handler {
	b := NewBuilder()
	if deps.Redis != nil {
		b.WithRedisChecker("redis", deps.Redis)
	}
	if deps.DB != nil {
		b.WithDBChecker("db", deps.DB)
	}
	return b.Build()
}

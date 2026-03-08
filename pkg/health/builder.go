package health

import "context"

// Builder 用于构建 Handler，支持链式调用注册 checker。
type Builder struct {
	checkers []Checker
}

// NewBuilder 创建 Builder 实例。
func NewBuilder() *Builder {
	return &Builder{checkers: []Checker{}}
}

// WithRedisChecker 注册 Redis 健康检查。
func (b *Builder) WithRedisChecker(name string, pinger Pinger) *Builder {
	b.checkers = append(b.checkers, PingChecker(name, pinger))
	return b
}

// WithChecker 注册自定义 Checker。
func (b *Builder) WithChecker(checker Checker) *Builder {
	b.checkers = append(b.checkers, checker)
	return b
}

// WithFuncChecker 注册基于函数的 Checker。
func (b *Builder) WithFuncChecker(name string, checkFunc func(context.Context) error) *Builder {
	b.checkers = append(b.checkers, &funcChecker{name: name, checkFunc: checkFunc})
	return b
}

// Build 构建 Handler。
func (b *Builder) Build() *Handler {
	return NewHandler(b.checkers...)
}

type funcChecker struct {
	name      string
	checkFunc func(context.Context) error
}

func (f *funcChecker) Name() string                    { return f.name }
func (f *funcChecker) Check(ctx context.Context) error { return f.checkFunc(ctx) }

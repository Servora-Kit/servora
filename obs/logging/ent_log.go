package logger

import (
	"fmt"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
)

func EntLogFuncFrom(logger kratoslog.Logger, module string) func(...any) {
	if zapLogger, ok := logger.(*ZapLogger); ok {
		moduleLogger := zapLogger.Zap().With(zap.String("module", module))
		return func(args ...any) {
			moduleLogger.Sugar().Debug(args...)
		}
	}

	helper := kratoslog.NewHelper(With(logger, WithModule(module)))
	return func(args ...any) {
		helper.Debug(fmt.Sprint(args...))
	}
}

package redis

import (
	"github.com/go-kratos/kratos/v2/log"
)

type testLogger struct{}

func (testLogger) Log(_ log.Level, _ ...interface{}) error { return nil }

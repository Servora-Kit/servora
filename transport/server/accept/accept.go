package accept

import (
	"errors"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// LoopConfig 控制 Loop 的 logger 和重试参数。
type LoopConfig struct {
	Logger     log.Logger
	RetryDelay time.Duration
}

// Loop 反复调用 accept 直到 listener 被关闭或不可恢复错误发生。
// 收到 net.ErrClosed 时静默返回；其它错误记一条日志并退出。
func Loop(cfg LoopConfig, accept func() error) {
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 50 * time.Millisecond
	}

	for {
		err := accept()
		if err == nil {
			continue
		}
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if cfg.Logger != nil {
			_ = cfg.Logger.Log(log.LevelError, "msg", "tcp accept failed", "err", err)
		}
		time.Sleep(retryDelay)
		return
	}
}

package accept

import (
	"errors"
	"net"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type AcceptLoopConfig struct {
	Logger     log.Logger
	RetryDelay time.Duration
}

func AcceptLoop(cfg AcceptLoopConfig, accept func() error) {
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

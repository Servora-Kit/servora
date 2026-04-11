package lifecycle

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type logRecorder struct {
	calls int
}

func (r *logRecorder) Log(_ log.Level, _ ...any) error {
	r.calls++
	return nil
}

func TestAcceptLoop_ReturnsOnClosedWithoutLogging(t *testing.T) {
	rec := &logRecorder{}
	calls := 0

	AcceptLoop(AcceptLoopConfig{Logger: rec}, func() error {
		calls++
		if calls == 1 {
			return nil
		}
		return net.ErrClosed
	})

	if calls != 2 {
		t.Fatalf("accept calls = %d, want 2", calls)
	}
	if rec.calls != 0 {
		t.Fatalf("logger calls = %d, want 0", rec.calls)
	}
}

func TestAcceptLoop_LogsAndReturnsOnError(t *testing.T) {
	rec := &logRecorder{}
	acceptErr := errors.New("accept failed")
	calls := 0

	AcceptLoop(AcceptLoopConfig{
		Logger:     rec,
		RetryDelay: time.Millisecond,
	}, func() error {
		calls++
		return acceptErr
	})

	if calls != 1 {
		t.Fatalf("accept calls = %d, want 1", calls)
	}
	if rec.calls != 1 {
		t.Fatalf("logger calls = %d, want 1", rec.calls)
	}
}

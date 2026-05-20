package accept

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

func TestLoop_ReturnsOnClosedWithoutLogging(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	calls := 0

	Loop(LoopConfig{Logger: l}, func() error {
		calls++
		if calls == 1 {
			return nil
		}
		return net.ErrClosed
	})

	if calls != 2 {
		t.Fatalf("accept calls = %d, want 2", calls)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output, got: %s", buf.String())
	}
}

func TestLoop_LogsAndReturnsOnError(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	acceptErr := errors.New("accept failed")
	calls := 0

	Loop(LoopConfig{
		Logger:     l,
		RetryDelay: time.Millisecond,
	}, func() error {
		calls++
		return acceptErr
	})

	if calls != 1 {
		t.Fatalf("accept calls = %d, want 1", calls)
	}
	if !strings.Contains(buf.String(), "tcp accept failed") {
		t.Fatalf("expected error log, got: %s", buf.String())
	}
}

package transporttest

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	ktransport "github.com/go-kratos/kratos/v2/transport"
)

func RoundTrip(t testing.TB, srv interface{}) {
	t.Helper()

	typed, ok := srv.(interface {
		ktransport.Server
		ktransport.Endpointer
	})
	if !ok {
		t.Fatalf("transporttest.RoundTrip expects server implementing transport.Server and transport.Endpointer, got %T", srv)
	}

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- typed.Start(context.Background())
	}()
	startReturned := false
	select {
	case err := <-startErrCh:
		startReturned = true
		if err != nil {
			t.Fatalf("start server: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
	}

	ep, err := typed.Endpoint()
	if err != nil {
		_ = typed.Stop(context.Background())
		t.Fatalf("resolve endpoint: %v", err)
	}
	if ep == nil {
		_ = typed.Stop(context.Background())
		t.Fatal("resolve endpoint: got nil endpoint")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := typed.Stop(stopCtx); err != nil {
		t.Fatalf("stop server: %v", err)
	}

	if !startReturned {
		select {
		case err := <-startErrCh:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("start server returned error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("start server did not return after stop")
		}
	}
}

package redis

import (
	"context"
	"testing"
	"time"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	cfg := &Config{Addr: "localhost:6379", DB: 15}
	c, cleanup, err := NewClient(cfg, testLogger{})
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(cleanup)
	t.Cleanup(func() { c.rdb.FlushDB(context.Background()) })
	return c
}

func TestTryLock_Success(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	lock, err := c.TryLock(ctx, "test:lock:1", 5*time.Second)
	if err != nil {
		t.Fatalf("expected lock acquired, got error: %v", err)
	}
	if lock == nil {
		t.Fatal("expected non-nil lock")
	}
	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("expected unlock success, got error: %v", err)
	}
}

func TestTryLock_AlreadyHeld(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	lock, err := c.TryLock(ctx, "test:lock:2", 5*time.Second)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}
	defer func() {
		if err := lock.Unlock(ctx); err != nil {
			t.Errorf("cleanup unlock failed: %v", err)
		}
	}()

	_, err = c.TryLock(ctx, "test:lock:2", 5*time.Second)
	if err != ErrLockNotAcquired {
		t.Fatalf("expected ErrLockNotAcquired, got %v", err)
	}
}

func TestUnlock_SafeRelease(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	lock, _ := c.TryLock(ctx, "test:lock:3", 5*time.Second)
	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("unlock should succeed: %v", err)
	}

	if err := lock.Unlock(ctx); err != ErrLockNotHeld {
		t.Fatalf("expected ErrLockNotHeld on double unlock, got %v", err)
	}
}

func TestUnlock_TokenMismatch(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	lock, _ := c.TryLock(ctx, "test:lock:4", 5*time.Second)

	fakeLock := &Lock{client: c, key: "test:lock:4", token: "wrong-token"}
	if err := fakeLock.Unlock(ctx); err != ErrLockNotHeld {
		t.Fatalf("expected ErrLockNotHeld for wrong token, got %v", err)
	}

	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("original holder should unlock successfully: %v", err)
	}
}

func TestTryLock_ExpiredThenReacquire(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	lock, _ := c.TryLock(ctx, "test:lock:5", 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	newLock, err := c.TryLock(ctx, "test:lock:5", 5*time.Second)
	if err != nil {
		t.Fatalf("should reacquire expired lock: %v", err)
	}
	defer func() {
		if err := newLock.Unlock(ctx); err != nil {
			t.Errorf("cleanup unlock failed: %v", err)
		}
	}()

	if err := lock.Unlock(ctx); err != ErrLockNotHeld {
		t.Fatalf("expired lock unlock should return ErrLockNotHeld, got %v", err)
	}
}

func TestTryLock_CancelledContext(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.TryLock(ctx, "test:lock:6", 5*time.Second)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

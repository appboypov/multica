package daemon

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalPathLockerSerializes(t *testing.T) {
	locker := NewLocalPathLocker()
	const path = "/some/path"

	rel1, err := locker.Acquire(context.Background(), path, "task-1", nil)
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	if got := locker.Holder(path); got != "task-1" {
		t.Errorf("holder = %q, want task-1", got)
	}

	// task-2 must wait, with onWait fired and the holder reported.
	var waitCalls atomic.Int32
	var sawHolder atomic.Value
	done := make(chan error, 1)
	go func() {
		rel, err := locker.Acquire(context.Background(), path, "task-2", func(holder string) {
			waitCalls.Add(1)
			sawHolder.Store(holder)
		})
		if err != nil {
			done <- err
			return
		}
		if got := locker.Holder(path); got != "task-2" {
			done <- errorsNew("holder after handover = " + got)
			return
		}
		rel()
		done <- nil
	}()

	// give the goroutine time to enter the wait
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && waitCalls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if waitCalls.Load() != 1 {
		t.Fatalf("onWait calls = %d, want 1", waitCalls.Load())
	}
	if got := sawHolder.Load(); got != "task-1" {
		t.Errorf("onWait holder = %v, want task-1", got)
	}

	rel1()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waiter result: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter never woke")
	}
	if got := locker.Holder(path); got != "" {
		t.Errorf("holder after release = %q, want empty", got)
	}
}

func TestLocalPathLockerCtxCancel(t *testing.T) {
	t.Parallel()

	locker := NewLocalPathLocker()
	const path = "/some/path"

	rel1, err := locker.Acquire(context.Background(), path, "task-1", nil)
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	defer rel1()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = locker.Acquire(ctx, path, "task-2", nil)
	if err == nil {
		t.Fatalf("expected ctx error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

func TestLocalPathLockerDistinctPathsParallel(t *testing.T) {
	locker := NewLocalPathLocker()

	rel1, err := locker.Acquire(context.Background(), "/a", "task-1", nil)
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	defer rel1()

	// Different path must not block.
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rel2, err := locker.Acquire(context.Background(), "/b", "task-2", nil)
		if err != nil {
			t.Errorf("acquire 2: %v", err)
			return
		}
		rel2()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("acquire on distinct path blocked")
	}
	wg.Wait()
}

// errorsNew is a tiny helper so the goroutine above can return a typed error
// without importing errors / fmt at the call site.
func errorsNew(msg string) error { return &waiterError{msg: msg} }

type waiterError struct{ msg string }

func (e *waiterError) Error() string { return e.msg }

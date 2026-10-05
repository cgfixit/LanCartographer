package probe

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 500 goroutines through a NewLimiter(3): observed peak concurrency must
// be exactly 3 — never above it, and the limit must actually be reached.
func TestLimiterConcurrencyBound(t *testing.T) {
	l := NewLimiter(3)

	var current, peak atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := l.Do(context.Background(), func() {
				c := current.Add(1)
				for {
					p := peak.Load()
					if c <= p || peak.CompareAndSwap(p, c) {
						break
					}
				}
				time.Sleep(time.Millisecond)
				current.Add(-1)
			})
			if err != nil {
				t.Errorf("Do returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := peak.Load(); got > 3 {
		t.Errorf("peak concurrency %d exceeded limit 3", got)
	}
	if got := peak.Load(); got < 3 {
		t.Errorf("peak concurrency %d never reached limit 3; limiter not saturating", got)
	}
}

// NewLimiter clamps: n <= 0 becomes the 128 default, n > 256 caps at 256.
func TestLimiterClamps(t *testing.T) {
	if got := cap(NewLimiter(0).sem); got != 128 {
		t.Errorf("NewLimiter(0) capacity = %d, want 128", got)
	}
	if got := cap(NewLimiter(-5).sem); got != 128 {
		t.Errorf("NewLimiter(-5) capacity = %d, want 128", got)
	}
	if got := cap(NewLimiter(999).sem); got != 256 {
		t.Errorf("NewLimiter(999) capacity = %d, want 256", got)
	}
	if got := cap(NewLimiter(7).sem); got != 7 {
		t.Errorf("NewLimiter(7) capacity = %d, want 7", got)
	}
}

// A waiter blocked on a full limiter must return ctx.Err() when its
// context is cancelled, and must not run f.
func TestLimiterCancelWhileWaiting(t *testing.T) {
	l := NewLimiter(1)

	release := make(chan struct{})
	started := make(chan struct{})
	go func() {
		_ = l.Do(context.Background(), func() {
			close(started)
			<-release
		})
	}()
	<-started // slot is now held

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- l.Do(ctx, func() {
			t.Error("f ran despite context cancellation")
		})
	}()

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Do returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Do did not return after context cancellation")
	}
	close(release)
}

// A panic inside f must still release the slot (defer release), while the
// panic itself propagates.
func TestLimiterPanicReleasesSlot(t *testing.T) {
	l := NewLimiter(1)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic to propagate")
			}
		}()
		_ = l.Do(context.Background(), func() { panic("boom") })
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ran := false
	if err := l.Do(ctx, func() { ran = true }); err != nil {
		t.Fatalf("Do after panicking f returned error: %v", err)
	}
	if !ran {
		t.Error("Do after panicking f did not run: slot was leaked")
	}
}

package winutil

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAwaitShareEnumAccountsForCancelledCalls(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()

	for range cap(shareEnumSlots) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		started := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			_, err := awaitShareEnum(ctx, func() ([]ShareInfo1, error) {
				close(started)
				<-release
				return nil, nil
			})
			done <- err
		}()
		select {
		case <-started:
		case err := <-done:
			cancel()
			t.Fatalf("enumeration failed to start: %v", err)
		case <-ctx.Done():
			cancel()
			t.Fatal("enumeration failed to start in time")
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("wait returned %v, want cancellation", err)
			}
		case <-time.After(time.Second):
			t.Fatal("cancel did not end the wait")
		}
	}
	if _, err := awaitShareEnum(t.Context(), func() ([]ShareInfo1, error) {
		return nil, nil
	}); err == nil {
		t.Fatal("another native call started while cancelled calls still held all slots")
	}
	unblock()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		shares, err := awaitShareEnum(t.Context(), func() ([]ShareInfo1, error) {
			return []ShareInfo1{{Name: "fixture"}}, nil
		})
		if err == nil && len(shares) == 1 && shares[0].Name == "fixture" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("enumeration slots did not recover after the calls finished")
}

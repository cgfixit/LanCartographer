package discover

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/probe"
)

var testIP = netip.MustParseAddr("192.168.1.42")

// runAlive calls Alive in a goroutine and fails the test if it does not
// return within the timeout.
func runAlive(t *testing.T, ctx context.Context, ip netip.Addr, lim *probe.Limiter, dial DialFunc, arp func(netip.Addr) bool) (bool, string) {
	t.Helper()
	type result struct {
		alive    bool
		foundVia string
	}
	ch := make(chan result, 1)
	go func() {
		a, v := Alive(ctx, ip, lim, dial, arp)
		ch <- result{a, v}
	}()
	select {
	case r := <-ch:
		return r.alive, r.foundVia
	case <-time.After(2 * time.Second):
		t.Fatal("Alive did not return within 2s")
		return false, ""
	}
}

func failDial(context.Context, netip.Addr, int) bool { return false }

func TestAlive_ARPOnly(t *testing.T) {
	alive, via := runAlive(t, context.Background(), testIP, probe.NewLimiter(0), failDial, func(netip.Addr) bool { return true })
	if !alive || via != "arp" {
		t.Fatalf("got (%v, %q), want (true, \"arp\")", alive, via)
	}
}

func TestAlive_OneTCPSuccess(t *testing.T) {
	dial := func(_ context.Context, _ netip.Addr, port int) bool { return port == 445 }
	alive, via := runAlive(t, context.Background(), testIP, probe.NewLimiter(0), dial, func(netip.Addr) bool { return false })
	if !alive || via != "tcp" {
		t.Fatalf("got (%v, %q), want (true, \"tcp\")", alive, via)
	}
}

func TestAlive_Dead(t *testing.T) {
	alive, via := runAlive(t, context.Background(), testIP, probe.NewLimiter(0), failDial, func(netip.Addr) bool { return false })
	if alive || via != "" {
		t.Fatalf("got (%v, %q), want (false, \"\")", alive, via)
	}
}

// TestAlive_ShortCircuitsOnSuccess has one fast successful dial while all
// other ports block until their ctx is cancelled. Alive must return promptly
// with ("tcp") and every blocked dial must observe the cancellation.
func TestAlive_ShortCircuitsOnSuccess(t *testing.T) {
	var blockedStarted, sawCancel atomic.Int32
	dial := func(ctx context.Context, _ netip.Addr, port int) bool {
		if port == 445 {
			return true
		}
		blockedStarted.Add(1)
		<-ctx.Done()
		sawCancel.Add(1)
		return false
	}
	start := time.Now()
	alive, via := runAlive(t, context.Background(), testIP, probe.NewLimiter(0), dial, func(netip.Addr) bool {
		t.Error("arp must not be consulted once tcp succeeded")
		return false
	})
	if !alive || via != "tcp" {
		t.Fatalf("got (%v, %q), want (true, \"tcp\")", alive, via)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Alive took %v; should return promptly after first success", d)
	}
	// Every port goroutine that actually started a blocking dial must have
	// been released by the cancellation (no goroutine left parked forever).
	deadline := time.After(2 * time.Second)
	for blockedStarted.Load() != sawCancel.Load() {
		select {
		case <-deadline:
			t.Fatalf("blocked dials never observed cancellation: started=%d cancelled=%d",
				blockedStarted.Load(), sawCancel.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// TestAlive_RespectsCallerCancellation cancels the caller's ctx while dials
// are blocked and expects a prompt return reporting not-alive.
func TestAlive_RespectsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	dial := func(ctx context.Context, _ netip.Addr, _ int) bool {
		wg.Add(1)
		defer wg.Done()
		<-ctx.Done()
		return false
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	alive, via := runAlive(t, ctx, testIP, probe.NewLimiter(0), dial, func(netip.Addr) bool { return false })
	if alive || via != "" {
		t.Fatalf("got (%v, %q), want (false, \"\") after cancellation", alive, via)
	}
	wg.Wait() // proves dial goroutines exited; -race catches misuse
}

func TestAlive_AllPortsProbed(t *testing.T) {
	var mu sync.Mutex
	seen := map[int]bool{}
	dial := func(_ context.Context, _ netip.Addr, port int) bool {
		mu.Lock()
		seen[port] = true
		mu.Unlock()
		return false
	}
	runAlive(t, context.Background(), testIP, probe.NewLimiter(0), dial, func(netip.Addr) bool { return false })
	mu.Lock()
	defer mu.Unlock()
	for _, p := range DiscoveryPorts {
		if !seen[p] {
			t.Errorf("port %d was never dialed", p)
		}
	}
}

func TestAlive_SelectedPortsDeduplicated(t *testing.T) {
	var mu sync.Mutex
	calls := map[int]int{}
	dial := func(_ context.Context, _ netip.Addr, port int) bool {
		mu.Lock()
		calls[port]++
		mu.Unlock()
		return false
	}
	Alive(context.Background(), testIP, probe.NewLimiter(2), dial, func(netip.Addr) bool { return false }, 50001, 50001, 445)
	if calls[50001] != 1 {
		t.Fatalf("selected port dialed %d times, want 1", calls[50001])
	}
	for _, port := range DiscoveryPorts {
		if calls[port] != 1 {
			t.Errorf("discovery port %d dialed %d times, want 1", port, calls[port])
		}
	}
}

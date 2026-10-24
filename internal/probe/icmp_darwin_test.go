//go:build darwin

package probe

import (
	"context"
	"net/netip"
	"testing"
	"time"
)

func TestICMPAlive(t *testing.T) {
	if !ICMPAlive(context.Background(), netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("owned loopback did not answer the OS ICMP probe")
	}
	if ICMPAlive(context.Background(), netip.MustParseAddr("::1")) {
		t.Fatal("IPv6 is outside the IPv4 discovery contract")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ICMPAlive(ctx, netip.MustParseAddr("127.0.0.1")) {
		t.Fatal("cancelled probe reported a reply")
	}
}

func TestICMPAlive_CallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if ICMPAlive(ctx, netip.MustParseAddr("127.0.0.2")) {
		t.Fatal("unused loopback address unexpectedly replied")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("probe ignored caller deadline: %s", elapsed)
	}
}

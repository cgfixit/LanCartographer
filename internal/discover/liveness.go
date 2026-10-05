package discover

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/local/lancartographer/internal/probe"
)

// DiscoveryPorts are the TCP ports probed to decide whether a host is
// alive. The set favors services common on Windows LANs (SMB, RDP, WinRM,
// web admin) plus SSH, ordered by expected hit rate.
var DiscoveryPorts = []int{445, 80, 443, 135, 139, 22, 3389, 8080, 8443}

// DialFunc makes one TCP connect attempt to ip:port, returning true on
// success. Implementations must honor ctx cancellation.
type DialFunc func(ctx context.Context, ip netip.Addr, port int) bool

// Alive reports whether ip responds on the LAN. All DiscoveryPorts are
// dialed concurrently, bounded by lim; the first successful dial ends the
// probe (foundVia "tcp") and cancels the remaining dials. If no port
// answers, arp is consulted as a fallback (foundVia "arp"). A host with no
// dial success and no ARP entry is reported (false, ""). Cancellation of
// the caller's ctx aborts the probe promptly.
func Alive(ctx context.Context, ip netip.Addr, lim *probe.Limiter, dial DialFunc, arp func(netip.Addr) bool) (alive bool, foundVia string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var success atomic.Bool
	var wg sync.WaitGroup
	for _, port := range DiscoveryPorts {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			if success.Load() {
				return
			}
			_ = lim.Do(ctx, func() {
				if dial(ctx, ip, port) {
					success.Store(true)
					cancel() // release dials still blocked in peers
				}
			})
		}(port)
	}
	wg.Wait()

	if success.Load() {
		return true, "tcp"
	}
	if arp(ip) {
		return true, "arp"
	}
	return false, ""
}

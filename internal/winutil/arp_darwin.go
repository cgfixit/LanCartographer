//go:build darwin

package winutil

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

// SendARP resolves the MAC address of the on-link IPv4 host at ip by reading
// the system ARP cache via `arp -n <ip>` with a 2s timeout. It only returns
// entries already in the cache, so a never-contacted host
// reports "no ARP entry" even when it is alive. A non-IPv4 ip is rejected
// before the exec.
func SendARP(ip netip.Addr) (mac [6]byte, err error) {
	if !ip.Is4() {
		return mac, fmt.Errorf("winutil: SendARP requires an IPv4 address, got %s", ip)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/sbin/arp", "-n", ip.String()).Output()
	// macOS reports an ordinary cache miss on stdout with a nonzero exit.
	if err != nil && strings.TrimSpace(string(out)) != fmt.Sprintf("%s (%s) -- no entry", ip, ip) {
		return mac, fmt.Errorf("winutil: SendARP(%s): arp lookup failed: %w", ip, err)
	}
	for line := range strings.Lines(string(out)) {
		entryMAC, entryIP, ok := parseARPEntry(line)
		if ok && entryIP == ip {
			return entryMAC, nil
		}
	}
	return mac, fmt.Errorf("winutil: SendARP(%s): no ARP entry", ip)
}

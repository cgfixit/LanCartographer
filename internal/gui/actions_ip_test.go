//go:build darwin || windows

package gui

import (
	"net/netip"
	"testing"

	"github.com/local/lancartographer/internal/model"
)

func TestDesktopActionsRequireIPv4(t *testing.T) {
	addresses := []netip.Addr{
		{},
		netip.MustParseAddr("::1"),
		netip.MustParseAddr("fe80::1%fixture"),
		netip.MustParseAddr("::ffff:127.0.0.1"),
	}
	for _, ip := range addresses {
		if got := pingArgv(ip); len(got) != 0 {
			t.Errorf("Ping accepted %v", ip)
		}
		for _, number := range []int{80, 3000} {
			h := model.Host{IP: ip, Ports: []model.Port{{Number: number, State: model.PortOpen, Service: "Web"}}}
			if target := browserURL(h); target != "" {
				t.Errorf("Browser accepted %v on port %d", ip, number)
			}
		}
	}
}

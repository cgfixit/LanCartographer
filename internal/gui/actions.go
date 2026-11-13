// Package gui is LanCartographer's Fyne interface: toolbar, live scan
// table, side panel, context menus, favorites, and export. Widget code
// stays thin; every launch or formatting decision lives here as a pure,
// unit-tested helper.
package gui

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
)

// themeModeLabel is the toolbar caption for the theme that is active.
// Anything other than "dark" is light, matching applyTheme.
func themeModeLabel(mode string) string {
	if mode == "dark" {
		return "Dark on"
	}
	return "Light on"
}

// browserURL selects an open web endpoint in the existing preference order.
// Preserve non-default ports: opening an 8080 service at port 80 is wrong.
func browserURL(h model.Host) string {
	for _, number := range []int{80, 8080, 443, 8443} {
		for _, p := range h.Ports {
			if p.Number != number || p.State != model.PortOpen {
				continue
			}
			scheme := "http"
			if number == 443 || number == 8443 {
				scheme = "https"
			}
			address := h.IP.String()
			if number == 8080 || number == 8443 {
				address = net.JoinHostPort(address, strconv.Itoa(number))
			}
			return scheme + "://" + address
		}
	}
	return ""
}

// parseMAC parses a colon-separated MAC string into a 6-byte hardware
// address. It rejects empty strings and the elevation sentinel.
func parseMAC(s string) (net.HardwareAddr, error) {
	if s == "" || s == probe.MacNeedsElevation {
		return nil, fmt.Errorf("no usable MAC address")
	}
	mac, err := net.ParseMAC(s)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC %q: %w", s, err)
	}
	if len(mac) != 6 {
		return nil, fmt.Errorf("MAC %q is not 6 bytes", s)
	}
	return mac, nil
}

// canWOL reports whether the host carries a usable MAC for Wake-on-LAN:
// present, not the elevation sentinel, and 6 bytes.
func canWOL(h model.Host) bool {
	_, err := parseMAC(h.MAC)
	return err == nil
}

// broadcastAddr derives the directed broadcast address of p (e.g.
// 10.0.0.0/24 -> 10.0.0.255) and appends the standard WOL discard port :9.
func broadcastAddr(p netip.Prefix) string {
	p = p.Masked()
	b := p.Addr().As4()
	for i := p.Bits(); i < 32; i++ {
		b[i/8] |= 1 << (7 - uint(i%8))
	}
	return netip.AddrFrom4(b).String() + ":9"
}

// sharesCell renders the table's shares column: "0 shares" when none,
// "access denied" when the only share is the synthetic denial record,
// otherwise the count plus the first share name.
func sharesCell(h model.Host) string {
	if len(h.Shares) == 0 {
		return "0 shares"
	}
	if len(h.Shares) == 1 && h.Shares[0].Name == "" && h.Shares[0].Remark == "shares: access denied" {
		return "access denied"
	}
	first := h.Shares[0].Name
	if first == "" {
		first = h.Shares[0].UNC
	}
	return fmt.Sprintf("%d shares (%s)", len(h.Shares), first)
}

// isDenialShare reports whether s is the synthetic access-denied record.
func isDenialShare(s model.Share) bool {
	return s.Name == "" && s.Remark == "shares: access denied"
}

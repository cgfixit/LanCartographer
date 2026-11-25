// Package gui is LanCartographer's Fyne interface: toolbar, live scan
// table, side panel, context menus, favorites, and export. Widget code
// stays thin; every launch or formatting decision lives here as a pure,
// unit-tested helper.
package gui

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
)

// themeModeLabel is the theme button caption. It names the theme the
// button switches TO, so the label reads as an action; anything other than
// "dark" is light, matching applyTheme.
func themeModeLabel(mode string) string {
	if mode == "dark" {
		return "Light theme"
	}
	return "Dark theme"
}

// plural renders a count with its noun: "1 host", "4 hosts".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// emptyCell marks a value the scan did not learn; it is displayed dimmed.
const emptyCell = "—"

// portsCell is the table's compact open-port list: "22 SSH, 443 Web,
// 18443". Only open ports are listed, so the state word is omitted; an
// unidentified service shows the number alone. The CLI keeps the longer
// model.OpenPortsSummary form.
func portsCell(h model.Host) string {
	var open []model.Port
	for _, p := range h.Ports {
		if p.State == model.PortOpen {
			open = append(open, p)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].Number < open[j].Number })
	parts := make([]string, 0, len(open))
	for _, p := range open {
		part := strconv.Itoa(p.Number)
		if p.Service != "" && p.Service != "unknown" {
			part += " " + p.Service
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// foundViaLabel explains how discovery saw the host.
func foundViaLabel(v string) string {
	switch v {
	case "tcp":
		return "TCP connection"
	case "arp":
		return "ARP cache"
	case "icmp":
		return "Ping (ICMP echo)"
	case "":
		return emptyCell
	}
	return v
}

// browserURL selects an open web endpoint in the existing preference order.
// Preserve non-default ports: opening an 8080 service at port 80 is wrong.
func browserURL(h model.Host) string {
	if !h.IP.Is4() {
		return ""
	}
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
	for _, p := range h.Ports {
		if p.State == model.PortOpen && p.Service == "Web" && p.Number >= 1 && p.Number <= 65535 {
			return "http://" + net.JoinHostPort(h.IP.String(), strconv.Itoa(p.Number))
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
		return ""
	}
	if len(h.Shares) == 1 && h.Shares[0].Name == "" && h.Shares[0].Remark == "shares: access denied" {
		return "access denied"
	}
	first := h.Shares[0].Name
	if first == "" {
		first = h.Shares[0].UNC
	}
	return fmt.Sprintf("%s (%s)", plural(len(h.Shares), "share"), first)
}

// isDenialShare reports whether s is the synthetic access-denied record.
func isDenialShare(s model.Share) bool {
	return s.Name == "" && s.Remark == "shares: access denied"
}

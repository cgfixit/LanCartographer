// Package gui is LanCartographer's Fyne interface: toolbar, live scan
// table, side panel, context menus, favorites, export, and (stubbed for
// now) the 2D map view. Widget code stays thin; every launch or formatting
// decision lives here as a pure, unit-tested helper.
package gui

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
)

// specPorts is the spec port set; it must include 445 (SMB) and 3389 (RDP).
const specPorts = "21,22,23,80,135,139,443,445,515,631,3389,5357,8080,8443,9100"

// parsePorts parses a comma-separated port list, deduplicating while
// preserving order. Entries must be integers in 1..65535.
func parsePorts(s string) ([]int, error) {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid port %q", part)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ports given")
	}
	return out, nil
}

// defaultPorts returns the spec port set as a slice.
func defaultPorts() []int {
	ports, err := parsePorts(specPorts)
	if err != nil {
		panic("spec port set must parse: " + err.Error())
	}
	return ports
}

// explorerPath builds the UNC path for a host (\\ip) or one of its shares
// (\\ip\share). A share whose Name is empty (the synthetic access-denied
// record) yields just the host path.
func explorerPath(ip netip.Addr, shareName string) string {
	if shareName == "" {
		return `\\` + ip.String()
	}
	return `\\` + ip.String() + `\` + shareName
}

// browserScheme picks the URL scheme for a host's web interface per spec:
// "http" when 80 or 8080 is open, "https" when 443 is open, "" otherwise.
func browserScheme(h model.Host) string {
	open := map[int]bool{}
	for _, p := range h.Ports {
		if p.State == model.PortOpen {
			open[p.Number] = true
		}
	}
	switch {
	case open[80] || open[8080]:
		return "http"
	case open[443]:
		return "https"
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

// pingArgv builds the argv that opens a visible console window running
// four pings against ip. ARGV LIST ONLY — never a shell string.
func pingArgv(ip netip.Addr) []string {
	return []string{"cmd", "/c", "start", "", "cmd", "/c", "ping", "-n", "4", ip.String()}
}

// rdpArgv builds the argv for an RDP session to ip.
func rdpArgv(ip netip.Addr) []string {
	return []string{"mstsc", "/v:" + ip.String()}
}

// browserArgv builds the argv that opens the host's web interface in the
// default browser. scheme must come from browserScheme ("" means no action
// and returns nil).
func browserArgv(ip netip.Addr, scheme string) []string {
	if scheme == "" {
		return nil
	}
	return []string{"cmd", "/c", "start", "", scheme + "://" + ip.String()}
}

// explorerArgv builds the argv that opens a host or share UNC in Explorer.
func explorerArgv(path string) []string {
	return []string{"explorer", path}
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

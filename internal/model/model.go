// Package model holds the inventory records for LanCartographer and the
// CSV/JSON export helpers that consume them.
package model

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

// Port states observed by a scan.
const (
	PortOpen    = "open"
	PortClosed  = "closed"
	PortTimeout = "timeout"
)

// Share types observed on SMB hosts.
const (
	ShareDisk  = "disk"
	SharePrint = "print"
	ShareIPC   = "ipc"
)

// Port is one probed TCP port on a host.
type Port struct {
	Number  int
	State   string
	Service string
	Title   string
}

// Share is one SMB share advertised by a host. UNC carries the full
// \\host\name path.
type Share struct {
	Name   string
	Type   string
	Remark string
	UNC    string
}

// Host is one discovered device on the LAN.
type Host struct {
	IP       netip.Addr
	Alive    bool
	FoundVia string
	Name     string
	MAC      string
	Vendor   string
	Ports    []Port
	Shares   []Share
}

// webPorts is the set of TCP ports classified as web services (must match
// the probe package's Web classification). Single home for HasWeb and the
// GUI globe badge.
var webPorts = []int{80, 443, 8080, 8443}

// HasWeb reports whether the host has an open web port (80, 443, 8080, 8443).
func (h Host) HasWeb() bool {
	for _, p := range h.Ports {
		if p.State != PortOpen {
			continue
		}
		for _, wp := range webPorts {
			if p.Number == wp {
				return true
			}
		}
	}
	return false
}

// HasRDP reports whether the host has RDP (port 3389) open.
func (h Host) HasRDP() bool {
	for _, p := range h.Ports {
		if p.State == PortOpen && p.Number == 3389 {
			return true
		}
	}
	return false
}

// OpenPortsSummary lists only the open ports, ascending, comma-space
// separated, e.g. "80 open http, 445 open smb". Empty when no port is open.
func (h Host) OpenPortsSummary() string {
	var open []Port
	for _, p := range h.Ports {
		if p.State == PortOpen {
			open = append(open, p)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].Number < open[j].Number })
	parts := make([]string, 0, len(open))
	for _, p := range open {
		parts = append(parts, fmt.Sprintf("%d %s %s", p.Number, p.State, p.Service))
	}
	return strings.Join(parts, ", ")
}

// SharesSummary reports the share count and the first share's UNC, e.g.
// `2 shares (first \\192.168.1.50\DATA)`, or "0 shares" when none.
func (h Host) SharesSummary() string {
	if len(h.Shares) == 0 {
		return "0 shares"
	}
	return fmt.Sprintf("%d shares (first %s)", len(h.Shares), h.Shares[0].UNC)
}

// WriteJSON writes hosts as an indented JSON array to w.
func WriteJSON(w io.Writer, hosts []Host) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(hosts)
}

var csvHeader = []string{"ip", "alive", "name", "mac", "vendor", "found_via", "ports", "shares"}

// WriteCSV writes a header row then one row per host. The ports and shares
// columns carry OpenPortsSummary()/SharesSummary() and are quoted by
// encoding/csv when they contain commas.
func WriteCSV(w io.Writer, hosts []Host) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, h := range hosts {
		row := []string{
			h.IP.String(),
			strconv.FormatBool(h.Alive),
			h.Name,
			h.MAC,
			h.Vendor,
			h.FoundVia,
			h.OpenPortsSummary(),
			h.SharesSummary(),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

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
	"time"
	"unicode/utf8"
)

// Port states observed by a scan.
const (
	PortOpen        = "open"
	PortClosed      = "closed"
	PortTimeout     = "timeout"
	PortUnconfirmed = "unconfirmed"
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
	// CertExpires is the RFC 3339 expiry of the certificate an HTTPS port
	// presented, empty when none was seen. CertTrusted is false when this
	// computer could not verify that certificate; its dates are then a
	// claim by an unauthenticated peer.
	CertExpires string `json:",omitempty"`
	CertTrusted bool   `json:",omitempty"`
}

// certSoon is how close an expiry must be before the days are spelled out.
const certSoon = 30 * 24 * time.Hour

// CertSummary describes the presented certificate for display, or returns
// "" when none was recorded.
func (p Port) CertSummary(now time.Time) string {
	expires, err := time.Parse(time.RFC3339, p.CertExpires)
	if err != nil {
		return ""
	}
	text := "certificate expires " + expires.Format(time.DateOnly)
	switch left := expires.Sub(now); {
	case left <= 0:
		text = "certificate expired " + expires.Format(time.DateOnly)
	case left <= certSoon:
		text += fmt.Sprintf(" (%d days)", int(left.Hours()/24))
	}
	if !p.CertTrusted {
		text += ", not trusted"
	}
	return text
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
// `2 shares (first \\192.168.1.50\DATA)`, "0 shares" when none, or access denied.
func (h Host) SharesSummary() string {
	if len(h.Shares) == 0 {
		return "0 shares"
	}
	if len(h.Shares) == 1 && h.Shares[0].Name == "" && h.Shares[0].Remark == "shares: access denied" {
		return h.Shares[0].Remark
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
// encoding/csv when they contain commas. Formula-like cells are prefixed
// with an apostrophe for spreadsheet import; JSON keeps the raw values.
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
		for i, cell := range row {
			row[i] = csvText(cell)
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// csvText neutralizes formula prefixes, including after whitespace and
// their full-width forms. CSV quoting alone does not stop evaluation.
func csvText(s string) string {
	first, _ := utf8.DecodeRuneInString(strings.TrimSpace(s))
	if strings.ContainsRune("=+-@＝＋－＠", first) || (s != "" && strings.ContainsAny(s[:1], "\t\r\n")) {
		return "'" + s
	}
	return s
}

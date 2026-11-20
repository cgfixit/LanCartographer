package macdiag

import (
	"net/netip"
	"strings"
	"testing"
)

const processRecord = "p812\x00cfixture\x00\n"

func TestConnectionsParse(t *testing.T) {
	rows := "f3\x00tIPv4\x00PTCP\x00n*:18080\x00TST=LISTEN\x00\n" +
		"f4\x00tIPv4\x00PTCP\x00n127.0.0.1:18080->127.0.0.2:55221\x00TST=ESTABLISHED\x00\n" +
		"f8\x00tIPv6\x00PUDP\x00n[fe80::8%en0]:5353\x00\n" +
		"p91\x00cname\x1b\x00\nf9\x00tIPv6\x00PTCP\x00n[::1]:18081->[:invalid]:999\x00\n"
	got := parseConnections([]byte(processRecord + rows))
	if !got.Partial || len(got.Rows) != 3 {
		t.Fatalf("expected three usable rows and a malformed peer, got %+v", got)
	}
	if r := got.Rows[0]; r.Local.String() != "*:18080" || r.HasPeer || r.State != "LISTEN" || r.Process != "fixture" || r.PID != 812 {
		t.Fatalf("wildcard listener = %+v", r)
	}
	if r := got.Rows[1]; !r.HasPeer || r.Peer.IP != netip.MustParseAddr("127.0.0.2") || r.Peer.Port != 55221 || r.State != "ESTABLISHED" {
		t.Fatalf("connected socket = %+v", r)
	}
	if r := got.Rows[2]; r.Local.String() != "[fe80::8%en0]:5353" || r.HasPeer || r.Protocol != "UDP" || r.PID != 812 {
		t.Fatalf("IPv6 UDP bind = %+v", r)
	}
}

func TestConnectionsMalformedAndCapped(t *testing.T) {
	const row = "f6\x00tIPv4\x00PUDP\x00n127.0.0.1:5\x00\n"
	for _, in := range []string{"\x00broken\x00\n" + row, row, processRecord + row[:len(row)-2]} {
		got := parseConnections([]byte(in))
		if !got.Partial || len(got.Rows) != 0 {
			t.Fatalf("bad record attributed as a socket: %+v", got)
		}
	}
	got := parseConnections([]byte(processRecord + strings.Repeat(row, maxConnections+1)))
	if !got.Partial || len(got.Rows) != maxConnections {
		t.Fatalf("row cap: partial=%v rows=%d", got.Partial, len(got.Rows))
	}
}

//go:build cgo

package gui

import (
	"net/netip"
	"strings"
	"testing"
)

func TestWiFiGatewayRoute(t *testing.T) {
	for _, tc := range []struct {
		route, want string
	}{
		{"gateway: 192.0.2.1\ninterface: en0", "192.0.2.1"},
		{"gateway: fe80::1\ninterface: en0", "fe80::1%en0"},
		{"gateway: fe80::1%en1\ninterface: en0", ""},
		{"gateway: 192.0.2.1\ninterface: utun0", ""},
		{"gateway: link#5\ninterface: en0", ""},
		{"gateway: 224.0.0.1\ninterface: en0", ""},
	} {
		addr := parseWiFiGateway(tc.route, "en0")
		if tc.want == "" && !addr.IsValid() {
			continue
		}
		if addr.String() != tc.want {
			t.Errorf("gateway %q = %v, want %q", tc.route, addr, tc.want)
		}
	}
}

func TestWiFiGatewayPing(t *testing.T) {
	for _, tc := range []struct {
		output  string
		replies int
		average float64
		ok      bool
	}{
		{"5 packets transmitted, 4 packets received, 20.0% packet loss\nround-trip min/avg/max/stddev = 1.0/2.5/4.0/0.3 ms", 4, 2.5, true},
		{"5 packets transmitted, 5 packets received, 0.0% packet loss\nround-trip min/avg/max/std-dev = 1/2/3/0.3 ms", 5, 2, true},
		{"5 packets transmitted, 0 packets received, 100.0% packet loss", 0, 0, true},
		{"5 packets transmitted, 5 packets received, 0.0% packet loss", 0, 0, false},
		{"1 packets transmitted, 1 packets received, 0.0% packet loss", 0, 0, false},
	} {
		gotReplies, gotAverage, ok := parseGatewayPing(tc.output)
		if ok != tc.ok || gotReplies != tc.replies || gotAverage != tc.average {
			t.Errorf("parse %q = (%d, %.2f, %t)", tc.output, gotReplies, gotAverage, ok)
		}
	}
}

func TestWiFiComparison(t *testing.T) {
	before := wifiSample{iface: "en0", gateway: netip.MustParseAddr("192.0.2.1"), signal: -70, replies: 5, averageMS: 12}
	after := before
	after.signal, after.averageMS = -64, 8
	if text := after.compare(before); !strings.Contains(text, "+6 dB") || !strings.Contains(text, "-4.0 ms") {
		t.Fatalf("comparison: %s", text)
	}
	after.gateway = netip.MustParseAddr("192.0.2.2")
	if text := after.compare(before); !strings.Contains(text, "separate samples") || strings.Contains(text, "dB") {
		t.Fatalf("changed gateway: %s", text)
	}
}

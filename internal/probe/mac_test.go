package probe

import (
	"net/netip"
	"testing"
)

func TestFormatMAC(t *testing.T) {
	cases := []struct {
		name string
		mac  [6]byte
		want string
	}{
		{"sequential", [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, "00:11:22:33:44:55"},
		{"upper hex lowercased", [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, "aa:bb:cc:dd:ee:ff"},
		{"zero mac", [6]byte{}, "00:00:00:00:00:00"},
		{"broadcast", [6]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, "ff:ff:ff:ff:ff:ff"},
		{"single nibble bytes zero-padded", [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "01:02:03:04:05:06"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatMAC(tc.mac); got != tc.want {
				t.Errorf("formatMAC(%x) = %q, want %q", tc.mac, got, tc.want)
			}
		})
	}
}

func TestLookupMAC_Unreachable(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1 (RFC 5737): guaranteed unroutable, so SendARP
	// must fail on Windows; off-Windows the stub always errors.
	mac, err := LookupMAC(netip.MustParseAddr("192.0.2.1"))
	if err == nil {
		t.Errorf("LookupMAC(192.0.2.1) = %q, nil; want error for unroutable ip", mac)
	}
	if mac != "" {
		t.Errorf("LookupMAC(192.0.2.1) mac = %q, want %q on error", mac, "")
	}
}

func TestLookupMAC_RejectsNonIPv4(t *testing.T) {
	// SendARP is IPv4-only; an IPv6 target must error rather than panic.
	mac, err := LookupMAC(netip.MustParseAddr("2001:db8::1"))
	if err == nil {
		t.Errorf("LookupMAC(2001:db8::1) = %q, nil; want error for non-IPv4 ip", mac)
	}
	if mac != "" {
		t.Errorf("LookupMAC(2001:db8::1) mac = %q, want %q on error", mac, "")
	}
}

package authz

import (
	"net/netip"
	"testing"
)

func TestNeedsConfirmation(t *testing.T) {
	local := []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}
	tests := []struct {
		name   string
		target netip.Prefix
		local  []netip.Prefix
		want   bool
	}{
		{"listed local /24 needs no confirmation", netip.MustParsePrefix("192.168.1.0/24"), local, false},
		{"listed local 10/8 still needs confirmation (broader than /22)", netip.MustParsePrefix("10.0.0.0/8"),
			[]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}, true},
		{"public range needs confirmation", netip.MustParsePrefix("203.0.113.0/24"), local, true},
		{"rfc1918 range outside local needs confirmation", netip.MustParsePrefix("192.168.99.0/24"), local, true},
		{"private range broader than /22 needs confirmation", netip.MustParsePrefix("192.168.0.0/16"), local, true},
		{"host bits masked into listed local needs no confirmation",
			netip.MustParsePrefix("192.168.1.5/24").Masked(), local, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeedsConfirmation(tc.target, tc.local); got != tc.want {
				t.Errorf("NeedsConfirmation(%v, %v) = %v, want %v", tc.target, tc.local, got, tc.want)
			}
		})
	}
}

func TestIsRFC1918(t *testing.T) {
	tests := []struct {
		prefix string
		want   bool
	}{
		{"10.1.0.0/16", true},
		{"172.16.0.0/12", true},
		{"172.31.255.0/24", true},
		{"192.168.5.0/24", true},
		{"172.15.0.0/16", false},
		{"172.32.0.0/12", false},
		{"203.0.113.0/24", false},
		{"8.8.8.0/24", false},
	}
	for _, tc := range tests {
		if got := IsRFC1918(netip.MustParsePrefix(tc.prefix)); got != tc.want {
			t.Errorf("IsRFC1918(%s) = %v, want %v", tc.prefix, got, tc.want)
		}
	}
}

func TestNoticeContainsCFAA(t *testing.T) {
	const want = "Scan only networks you own or have explicit authorization to inventory. " +
		"Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws."
	if Notice != want {
		t.Errorf("Notice mismatch:\n got: %q\nwant: %q", Notice, want)
	}
}

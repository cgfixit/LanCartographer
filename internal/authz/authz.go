// Package authz holds the authorization notice and scan-range
// confirmation policy for LanCartographer.
package authz

import "net/netip"

// Notice is shown to the operator before any scan.
const Notice = "Scan only networks you own or have explicit authorization to inventory. " +
	"Unauthorized access or scanning may violate the Computer Fraud and Abuse Act, 18 U.S.C. § 1030, and similar laws."

var rfc1918 = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
}

// IsRFC1918 reports whether p falls inside RFC1918 private address space
// (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16).
func IsRFC1918(p netip.Prefix) bool {
	addr := p.Masked().Addr()
	for _, r := range rfc1918 {
		if r.Contains(addr) {
			return true
		}
	}
	return false
}

// NeedsConfirmation reports whether scanning target requires explicit
// operator confirmation. Callers must Masked() the target first.
//
// A target fully contained in a known local prefix is already authorized
// and never needs confirmation. Any other target needs confirmation when
// it is outside RFC1918 space or broader than a /22.
func NeedsConfirmation(target netip.Prefix, local []netip.Prefix) bool {
	for _, l := range local {
		if l.Bits() <= target.Bits() && l.Contains(target.Addr()) {
			return false
		}
	}
	return !IsRFC1918(target) || target.Bits() < 22
}

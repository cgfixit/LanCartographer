// Package oui resolves the vendor of a MAC address from a small embedded,
// offline OUI subset (data.csv, regenerated into oui_data.go via
// "go generate"). It never downloads anything; unknown prefixes return "".
package oui

//go:generate go run gen.go

// Lookup returns the vendor name for a MAC address, or "" when the MAC is
// malformed, shorter than 3 bytes, or its prefix is not in the embedded
// subset. Accepted formats include "3c:5a:b4:00:00:00", "3C-5A-B4-00-00-00",
// "3c5a.b400.0000", and "3c5ab4000000"; separators, case, and trailing bytes
// beyond the 3-byte prefix are ignored.
func Lookup(mac string) string {
	hex := make([]byte, 0, len(mac))
	for i := 0; i < len(mac); i++ {
		c := mac[i]
		switch {
		case c == ':' || c == '-' || c == '.':
			continue
		case c >= '0' && c <= '9':
			hex = append(hex, c)
		case c >= 'a' && c <= 'f':
			hex = append(hex, c-'a'+'A')
		case c >= 'A' && c <= 'F':
			hex = append(hex, c)
		default:
			return ""
		}
	}
	if len(hex) < 6 {
		return ""
	}
	return vendorByPrefix[string(hex[:6])]
}

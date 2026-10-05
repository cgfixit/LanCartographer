package oui

import "testing"

func TestLookupFormats(t *testing.T) {
	// data.csv row: 3C5AB4,Google — keep expectation in sync with the CSV.
	const want = "Google"
	for _, mac := range []string{
		"3c:5a:b4:00:00:00",
		"3C:5A:B4:00:00:00",
		"3C-5A-B4-00-00-00",
		"3c5ab4000000",
		"3C5AB4000000",
		"3c5a.b400.0000",
	} {
		if got := Lookup(mac); got != want {
			t.Errorf("Lookup(%q) = %q, want %q", mac, got, want)
		}
	}
}

func TestLookupUnknownOrMalformed(t *testing.T) {
	for _, mac := range []string{
		"",
		"3c",
		"3c:5a",
		"zz:zz:zz:zz:zz:zz",
		"not-a-mac",
		"FF:FF:FF:00:00:00", // valid MAC, unknown prefix
		"01:00:5e:00:00:00", // multicast prefix not in the subset
	} {
		if got := Lookup(mac); got != "" {
			t.Errorf("Lookup(%q) = %q, want empty", mac, got)
		}
	}
}

func TestLookupKnownSubset(t *testing.T) {
	cases := map[string]string{
		"00:00:0c:aa:bb:cc": "Cisco",
		"00:11:32:01:02:03": "Synology",
		"b8:27:eb:de:ad:00": "Raspberry Pi",
		"24-0A-C4-11-22-33": "Espressif",
	}
	for mac, want := range cases {
		if got := Lookup(mac); got != want {
			t.Errorf("Lookup(%q) = %q, want %q", mac, got, want)
		}
	}
}

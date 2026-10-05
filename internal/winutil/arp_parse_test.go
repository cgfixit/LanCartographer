package winutil

import (
	"net/netip"
	"testing"
)

func TestParseARPEntry(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantIP  string
		wantMAC [6]byte
	}{
		{
			name:    "resolved with hostname",
			line:    "host.example (192.168.1.5) at d8:f2:ca:e0:38:8c on en0 ifscope",
			wantOK:  true,
			wantIP:  "192.168.1.5",
			wantMAC: [6]byte{0xd8, 0xf2, 0xca, 0xe0, 0x38, 0x8c},
		},
		{
			name:    "question-mark name, single-digit octets, trailing flag",
			line:    "? (10.0.0.1) at 0:11:22:33:44:55 on en0 ifscope [ethernet]",
			wantOK:  true,
			wantIP:  "10.0.0.1",
			wantMAC: [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		},
		{
			name:    "resolved on secondary interface",
			line:    "router (172.16.0.254) at a0:b1:c2:d3:e4:f5 on en1",
			wantOK:  true,
			wantIP:  "172.16.0.254",
			wantMAC: [6]byte{0xa0, 0xb1, 0xc2, 0xd3, 0xe4, 0xf5},
		},
		{
			name:   "incomplete entry rejected",
			line:   "192.168.1.5 (192.168.1.5) at (incomplete) on en0 ifscope",
			wantOK: false,
		},
		{
			name:   "IPv6 neighbour entry rejected",
			line:   "fe80::1 (fe80::1) at d8:f2:ca:e0:38:8c on en0 ifscope",
			wantOK: false,
		},
		{
			name:   "bad MAC rejected",
			line:   "host (192.168.1.5) at zz:zz:zz:zz:zz:zz on en0",
			wantOK: false,
		},
		{
			name:   "garbage line rejected",
			line:   "not an arp line at all",
			wantOK: false,
		},
		{
			name:   "empty line rejected",
			line:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac, ip, ok := parseARPEntry(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseARPEntry(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if ip != netip.MustParseAddr(tt.wantIP) {
				t.Errorf("ip = %s, want %s", ip, tt.wantIP)
			}
			if mac != tt.wantMAC {
				t.Errorf("mac = %x, want %x", mac, tt.wantMAC)
			}
		})
	}
}

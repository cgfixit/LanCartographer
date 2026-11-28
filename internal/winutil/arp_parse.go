package winutil

import (
	"net/netip"
	"strconv"
	"strings"
)

// parseARPEntry parses one line of macOS `arp` output. It handles the
// resolved form:
//
//	host.example (192.168.1.5) at d8:f2:ca:e0:38:8c on en0 ifscope [ethernet]
//
// and rejects the unresolved form (`at (incomplete)`). macOS prints MAC
// octets unpadded ("0:11:22:33:44:55"), which net.ParseMAC rejects, so the
// MAC is parsed octet-by-octet. ok is false when the line carries neither a
// usable IPv4 address nor a usable MAC. It is pure (no I/O) so it is
// table-tested on any OS.
func parseARPEntry(line string) (mac [6]byte, ip netip.Addr, ok bool) {
	open := strings.IndexByte(line, '(')
	closeParen := strings.IndexByte(line, ')')
	if open < 0 || closeParen <= open+1 {
		return mac, ip, false
	}
	addr, err := netip.ParseAddr(line[open+1 : closeParen])
	if err != nil || !addr.Is4() {
		return mac, ip, false
	}
	fields := strings.Fields(line[closeParen+1:])
	if len(fields) < 2 || fields[0] != "at" {
		return mac, ip, false
	}
	mac, ok = parseARPMAC(fields[1])
	if !ok {
		return mac, ip, false
	}
	return mac, addr, true
}

// parseARPMAC parses six colon-separated hex octets, each one or two digits
// (macOS pads nothing), into a hardware address.
func parseARPMAC(s string) (mac [6]byte, ok bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return mac, false
	}
	for i, p := range parts {
		if len(p) == 0 || len(p) > 2 {
			return mac, false
		}
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return mac, false
		}
		mac[i] = byte(v)
	}
	return mac, true
}

package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// mdnsPort is the mDNS service port (UDP).
const mdnsPort = 5353

// mdnsGroup is the link-local mDNS multicast group.
var mdnsGroup = netip.MustParseAddr("224.0.0.251")

// mdnsTimeout bounds the whole mDNS exchange, dial through the final read.
const mdnsTimeout = 800 * time.Millisecond

// mdnsMaxResponses caps how many replies are inspected before giving up.
const mdnsMaxResponses = 4

// MDNSName asks ip for its mDNS PTR name (RFC 6762 style): one PTR query for
// the address's in-addr.arpa name is sent both to the 224.0.0.251 multicast
// group and unicast straight to ip:5353 — Windows Wi-Fi multicast delivery
// is unreliable enough that the unicast copy catches responders either way.
// The first answer whose source address is ip wins; the rdata hostname is
// returned with any ".local" suffix stripped. Every failure — timeout, no
// listener, malformed packet, wrong source — yields "", never an error: a
// missing name is an empty name, not an error row.
func MDNSName(ctx context.Context, ip netip.Addr) string {
	var id [2]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, mdnsTimeout)
	defer cancel()

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return ""
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	query := buildMDNSQuery(binary.BigEndian.Uint16(id[:]), ip)
	group := &net.UDPAddr{IP: net.ParseIP(mdnsGroup.String()), Port: mdnsPort}
	unicast := &net.UDPAddr{IP: net.ParseIP(ip.String()), Port: mdnsPort}
	if _, err := conn.WriteToUDP(query, group); err != nil {
		return ""
	}
	if _, err := conn.WriteToUDP(query, unicast); err != nil {
		return ""
	}

	buf := make([]byte, 2048)
	for i := 0; i < mdnsMaxResponses; i++ {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			return ""
		}
		srcIP, ok := netip.AddrFromSlice(src.IP.To4())
		if !ok {
			continue
		}
		if name, ok := parsePTRName(buf[:n], srcIP, ip); ok {
			return name
		}
	}
	return ""
}

// buildMDNSQuery builds a standard DNS query for the PTR record of ip's
// in-addr.arpa name: header with the given transaction ID, zero flags
// (mDNS responders answer plain IN queries; no RD/QU games needed), QD=1,
// no other sections, then the reversed-digits question with type PTR
// (0x000C) and class IN (0x0001).
func buildMDNSQuery(id uint16, ip netip.Addr) []byte {
	b := make([]byte, 0, 12+30)
	b = binary.BigEndian.AppendUint16(b, id)
	b = binary.BigEndian.AppendUint16(b, 0x0000) // flags
	b = binary.BigEndian.AppendUint16(b, 1)      // QDCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ANCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // NSCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ARCOUNT
	a4 := ip.As4()
	for i := 3; i >= 0; i-- { // reverse the dotted quad
		digits := strconv.Itoa(int(a4[i]))
		b = append(b, byte(len(digits)))
		b = append(b, digits...)
	}
	b = append(b, byte(len("in-addr")))
	b = append(b, "in-addr"...)
	b = append(b, byte(len("arpa")))
	b = append(b, "arpa"...)
	b = append(b, 0x00)                          // root label
	b = binary.BigEndian.AppendUint16(b, 0x000C) // PTR
	b = binary.BigEndian.AppendUint16(b, 0x0001) // IN
	return b
}

// parsePTRName extracts the first PTR answer's rdata hostname from an mDNS
// response. src must equal want — a PTR from anyone but the host we asked
// is noise on a busy multicast segment, never a name. Owner names and rdata
// may use 0xC0 compression pointers; any structural failure yields ("", false).
func parsePTRName(resp []byte, src, want netip.Addr) (string, bool) {
	if src != want || len(resp) < 12 {
		return "", false
	}
	ancount := int(binary.BigEndian.Uint16(resp[6:8]))
	off := 12
	// Question section: QDCOUNT names, each 2-byte type + 2-byte class.
	for i := int(binary.BigEndian.Uint16(resp[4:6])); i > 0; i-- {
		var ok bool
		off, ok = skipDNSName(resp, off)
		if !ok || len(resp) < off+4 {
			return "", false
		}
		off += 4
	}
	for i := 0; i < ancount; i++ {
		var ok bool
		off, ok = skipDNSName(resp, off)
		if !ok || len(resp) < off+10 {
			return "", false
		}
		typ := binary.BigEndian.Uint16(resp[off : off+2])
		rdlen := int(binary.BigEndian.Uint16(resp[off+8 : off+10]))
		off += 10
		if len(resp) < off+rdlen {
			return "", false
		}
		if typ != 0x000C { // not PTR: skip its rdata
			off += rdlen
			continue
		}
		name, n := readDNSName(resp, off)
		if n == 0 {
			return "", false
		}
		name = strings.TrimSuffix(name, ".local.")
		name = strings.TrimSuffix(name, ".local")
		return name, true
	}
	return "", false
}

// skipDNSName advances past a DNS name at off (labels plus terminator, with
// 0xC0 compression pointers allowed) and returns the offset after it.
func skipDNSName(b []byte, off int) (int, bool) {
	_, n := readDNSName(b, off)
	if n == 0 {
		return 0, false
	}
	return off + n, true
}

// readDNSName decodes the DNS name at off, following 0xC0 pointers (with a
// jump cap so a corrupt packet cannot loop). It returns the decoded name in
// dotted form with a trailing dot (e.g. "printer.local.") and the number of
// bytes the name occupied at off, or ("", 0) on structural failure.
func readDNSName(b []byte, off int) (string, int) {
	if off < 0 || off >= len(b) {
		return "", 0
	}
	var sb strings.Builder
	consumed := 0
	pos := off
	for jumps := 0; ; jumps++ {
		if jumps > 16 || pos >= len(b) {
			return "", 0
		}
		l := int(b[pos])
		switch {
		case l == 0:
			pos++
			if consumed == 0 {
				consumed = pos - off
			}
			return sb.String(), consumed
		case l&0xC0 == 0xC0:
			if pos+1 >= len(b) {
				return "", 0
			}
			ptr := int(l&0x3F)<<8 | int(b[pos+1])
			if consumed == 0 {
				consumed = pos + 2 - off
			}
			pos = ptr
		case l&0xC0 != 0 || pos+1+l > len(b):
			return "", 0
		default:
			pos++
			sb.Write(b[pos : pos+l])
			sb.WriteByte('.')
			pos += l
		}
	}
}

package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net"
	"net/netip"
	"strings"
	"time"
)

// nbPort is the NetBIOS name service port.
const nbPort = 137

// nbTimeout bounds the whole node-status exchange, dial through read.
const nbTimeout = 800 * time.Millisecond

// NetBIOSName queries ip's NetBIOS name service (UDP 137) with a wildcard
// node-status request and returns the first registered 15-character name,
// trailing spaces trimmed. Any failure — no listener, timeout, malformed or
// mismatched reply — yields "", never an error: a missing name is an empty
// name, not an error row. The exchange is bounded by nbTimeout and by ctx,
// whichever fires first.
func NetBIOSName(ctx context.Context, ip netip.Addr) string {
	return nbNodeStatus(ctx, netip.AddrPortFrom(ip, nbPort))
}

// nbNodeStatus is the testable core of NetBIOSName against an arbitrary
// UDP endpoint.
func nbNodeStatus(ctx context.Context, addr netip.AddrPort) string {
	var id [2]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, nbTimeout)
	defer cancel()

	d := net.Dialer{Timeout: nbTimeout}
	conn, err := d.DialContext(ctx, "udp", addr.String())
	if err != nil {
		return ""
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	if _, err := conn.Write(nbQuery(id)); err != nil {
		return ""
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return ""
	}
	resp := buf[:n]
	if len(resp) < 2 || resp[0] != id[0] || resp[1] != id[1] {
		return ""
	}
	return parseNodeStatus(resp)
}

// nbQuery builds a wildcard node-status request: transaction ID, zero flags,
// QDCOUNT 1, then the encoded "*" name with type NBSTAT (0x0021), class IN.
func nbQuery(id [2]byte) []byte {
	var b []byte
	b = append(b, id[:]...)
	b = binary.BigEndian.AppendUint16(b, 0x0000) // flags
	b = binary.BigEndian.AppendUint16(b, 1)      // QDCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ANCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // NSCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ARCOUNT
	b = append(b, 0x20)
	b = append(b, encodeNBName("*")...)
	b = append(b, 0x00) // root label
	b = binary.BigEndian.AppendUint16(b, 0x0021)
	b = binary.BigEndian.AppendUint16(b, 0x0001)
	return b
}

// encodeNBName applies NetBIOS first-level encoding (RFC 1002 §4.1) to a
// 16-byte name field: each byte becomes two bytes, 'A'+high nibble then
// 'A'+low nibble. name is NUL-padded to 16 bytes or truncated to fit.
func encodeNBName(name string) []byte {
	out := make([]byte, 0, 32)
	for i := 0; i < 16; i++ {
		var c byte
		if i < len(name) {
			c = name[i]
		}
		out = append(out, 'A'+(c>>4), 'A'+(c&0x0F))
	}
	return out
}

// decodeNBName is the inverse of encodeNBName: pairs of 'A'+nibble bytes
// decode to raw bytes. Input must be non-empty, even-length, and use only
// bytes 'A' through 'P'; anything else yields "".
func decodeNBName(b []byte) string {
	if len(b) == 0 || len(b)%2 != 0 {
		return ""
	}
	out := make([]byte, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		hi, lo := b[i], b[i+1]
		if hi < 'A' || hi > 'P' || lo < 'A' || lo > 'P' {
			return ""
		}
		out[i/2] = (hi-'A')<<4 | (lo - 'A')
	}
	return string(out)
}

// parseNodeStatus extracts the first 15-character name from a NODE STATUS
// RESPONSE (RFC 1002 §4.2.18). Matching the transaction ID is the caller's
// job; this checks structure only: a parseable header, question, answer
// header, and an rdata section holding at least count complete 18-byte name
// entries (15 name bytes, 1 suffix byte, 2 flag bytes). It tolerates literal
// or pointer answer names, scope labels in the question, and trailing bytes
// beyond rdlength, but rejects anything structurally short with "".
func parseNodeStatus(resp []byte) string {
	if len(resp) < 12 {
		return ""
	}
	off := 12 // header: id, flags, 4 counts

	// Question section: one name (possibly with scope labels), type, class.
	off = skipNBName(resp, off)
	if off < 0 || len(resp) < off+4 {
		return ""
	}
	off += 4

	// Answer: name as compression pointer or literal labels.
	if len(resp) < off+2 {
		return ""
	}
	if resp[off]&0xC0 == 0xC0 {
		off += 2
	} else {
		off = skipNBName(resp, off)
		if off < 0 {
			return ""
		}
	}

	// type(2), class(2), ttl(4), rdlength(2)
	if len(resp) < off+10 {
		return ""
	}
	rdlen := int(binary.BigEndian.Uint16(resp[off+8 : off+10]))
	off += 10
	if rdlen < 1 || len(resp) < off+rdlen {
		return ""
	}
	rdata := resp[off : off+rdlen]
	count := int(rdata[0])
	if count == 0 || len(rdata) < 1+18*count {
		return ""
	}
	name := rdata[1 : 1+15]
	return strings.TrimRight(string(name), " \x00")
}

// skipNBName advances past a DNS-format name (a run of length-prefixed
// labels ending in a zero byte) starting at off. Returns the offset after
// the terminator, or -1 if the name is structurally invalid.
func skipNBName(b []byte, off int) int {
	for {
		if off >= len(b) {
			return -1
		}
		l := int(b[off])
		off++
		if l == 0 {
			return off
		}
		if l > 63 || len(b) < off+l {
			return -1
		}
		off += l
	}
}

// reverseResolver is the stdlib resolver ReverseName goes through; tests
// can swap it for a fake so no real PTR traffic is needed.
var reverseResolver = &net.Resolver{}

// ReverseName returns the first DNS label of ip's PTR record, lowercased,
// or "" if the lookup fails, times out, or returns nothing. The caller's
// ctx bounds the wait — the engine gives it 1.5s so a slow or absent
// resolver cannot stall enrichment.
func ReverseName(ctx context.Context, ip netip.Addr) string {
	names, err := reverseResolver.LookupAddr(ctx, ip.String())
	if err != nil || len(names) == 0 {
		return ""
	}
	label := names[0]
	if i := strings.IndexByte(label, '.'); i >= 0 {
		label = label[:i]
	}
	return strings.ToLower(label)
}

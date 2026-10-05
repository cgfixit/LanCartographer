package probe

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// nbName returns the 15-byte wire form of a NetBIOS name, space-padded.
func nbName(s string) [15]byte {
	var b [15]byte
	for i := range b {
		b[i] = ' '
	}
	copy(b[:], s)
	return b
}

// nbNameNUL returns the 15-byte wire form NUL-padded, a real-world quirk.
func nbNameNUL(s string) [15]byte {
	var b [15]byte
	copy(b[:], s)
	return b
}

type nbEntry struct {
	name   [15]byte
	suffix byte
	flags  uint16
}

// nodeStatusResponse builds a NODE STATUS RESPONSE fixture packet with the
// given transaction ID and name entries. The answer name is a compression
// pointer back to the question, as real responders send.
func nodeStatusResponse(id uint16, entries []nbEntry) []byte {
	var b []byte
	b = binary.BigEndian.AppendUint16(b, id)
	b = binary.BigEndian.AppendUint16(b, 0x8500) // response, authoritative
	b = binary.BigEndian.AppendUint16(b, 1)      // QDCOUNT
	b = binary.BigEndian.AppendUint16(b, 1)      // ANCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // NSCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ARCOUNT
	b = append(b, 0x20)
	b = append(b, encodeNBName("*")...)
	b = append(b, 0x00) // root label, no scope
	b = binary.BigEndian.AppendUint16(b, 0x0021)
	b = binary.BigEndian.AppendUint16(b, 0x0001)
	b = append(b, 0xC0, 0x0C) // answer name: pointer to question name
	b = binary.BigEndian.AppendUint16(b, 0x0021)
	b = binary.BigEndian.AppendUint16(b, 0x0001)
	b = binary.BigEndian.AppendUint32(b, 300) // TTL
	b = binary.BigEndian.AppendUint16(b, uint16(1+18*len(entries)))
	b = append(b, byte(len(entries)))
	for _, e := range entries {
		b = append(b, e.name[:]...)
		b = append(b, e.suffix)
		b = binary.BigEndian.AppendUint16(b, e.flags)
	}
	return b
}

func TestEncodeNBName(t *testing.T) {
	if got := encodeNBName("*"); string(got) != "CKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Errorf(`encodeNBName("*") = %q, want "CKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`, got)
	}
	if got := encodeNBName("FOO"); string(got) != "EGEPEP"+strings.Repeat("AA", 13) {
		t.Errorf(`encodeNBName("FOO") = %q, want "EGEPEP"+13*"AA"`, got)
	}
}

func TestDecodeNBName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"wildcard encoded", "CKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "*" + strings.Repeat("\x00", 15)},
		{"FOO encoded", "EGEPEP" + strings.Repeat("AA", 13), "FOO" + strings.Repeat("\x00", 13)},
		{"odd length", "ABC", ""},
		{"byte above P", "EGEPEQ", ""},
		{"lowercase not valid", "egepep", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeNBName([]byte(tc.input)); got != tc.want {
				t.Errorf("decodeNBName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNBNameRoundTrip(t *testing.T) {
	names := []string{"", "A", "DESKTOP-ABC", "ABCDEFGHIJKLMNOP", "trailing\x00bytes\x00\x00"}
	for _, name := range names {
		want := name
		for len(want) < 16 {
			want += "\x00"
		}
		if len(want) > 16 {
			want = want[:16]
		}
		if got := decodeNBName(encodeNBName(name)); got != want {
			t.Errorf("decode(encode(%q)) = %q, want %q", name, got, want)
		}
	}
}

func TestParseNodeStatus(t *testing.T) {
	valid := nodeStatusResponse(0x1234, []nbEntry{
		{name: nbName("DESKTOP-ABC"), suffix: 0x00, flags: 0x0400},
		{name: nbName("WORKGROUP"), suffix: 0x00, flags: 0x8400},
		{name: nbName("DESKTOP-ABC"), suffix: 0x20, flags: 0x0400},
	})

	// rdlengthCorrupt claims a single rdata byte while a name entry follows.
	rdlengthCorrupt := nodeStatusResponse(0x1234, []nbEntry{{name: nbName("DESKTOP-ABC")}})
	// rdlength sits 10 bytes before the count byte; find it by parsing the
	// fixed layout: 12 header + 38 question + 2 pointer + 2+2+4.
	rdlenOff := 12 + 38 + 2 + 8
	rdlengthCorrupt[rdlenOff] = 0
	rdlengthCorrupt[rdlenOff+1] = 1

	cases := []struct {
		name string
		resp []byte
		want string
	}{
		{"valid multi-entry returns first", valid, "DESKTOP-ABC"},
		{"single entry", nodeStatusResponse(0x9, []nbEntry{{name: nbName("NAS"), suffix: 0x00}}), "NAS"},
		{"NUL-padded name trimmed", nodeStatusResponse(0x9, []nbEntry{{name: nbNameNUL("SERVER1"), suffix: 0x00}}), "SERVER1"},
		{"truncated header", valid[:5], ""},
		{"truncated question", valid[:20], ""},
		{"truncated mid-entry", valid[:len(valid)-4], ""},
		{"garbage", []byte("not a netbios packet at all, just noise bytes"), ""},
		{"empty", nil, ""},
		{"count zero", nodeStatusResponse(0x9, nil), ""},
		{"rdlength smaller than entries", rdlengthCorrupt, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseNodeStatus(tc.resp); got != tc.want {
				t.Errorf("parseNodeStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseNodeStatus_LiteralAnswerName covers responders that echo the full
// encoded name in the answer instead of a compression pointer.
func TestParseNodeStatus_LiteralAnswerName(t *testing.T) {
	withPointer := nodeStatusResponse(0x9, []nbEntry{{name: nbName("DESKTOP-ABC"), suffix: 0x00}})
	// Replace the 2-byte pointer at offset 12+38 with the literal
	// 0x20 + 32 encoded bytes + 0x00 name form.
	var b []byte
	b = append(b, withPointer[:12+38]...)
	b = append(b, 0x20)
	b = append(b, encodeNBName("*")...)
	b = append(b, 0x00)
	b = append(b, withPointer[12+38+2:]...)
	if got := parseNodeStatus(b); got != "DESKTOP-ABC" {
		t.Errorf("parseNodeStatus(literal answer name) = %q, want %q", got, "DESKTOP-ABC")
	}
}

// TestParseNodeStatus_QuestionWithScope covers a question name carrying a
// scope label between the encoded name and the root terminator.
func TestParseNodeStatus_QuestionWithScope(t *testing.T) {
	base := nodeStatusResponse(0x9, []nbEntry{{name: nbName("DESKTOP-ABC"), suffix: 0x00}})
	var b []byte
	b = append(b, base[:12]...)      // header
	b = append(b, base[12:12+33]...) // 0x20 + encoded name
	b = append(b, 5)
	b = append(b, "SCOPE"...)
	b = append(b, 0x00)
	b = append(b, base[12+33+1:]...) // qtype onward, skipping old terminator
	if got := parseNodeStatus(b); got != "DESKTOP-ABC" {
		t.Errorf("parseNodeStatus(question with scope) = %q, want %q", got, "DESKTOP-ABC")
	}
}

// nbFixture starts a UDP listener on loopback that passes each request to
// respond and writes back its result (nil = no reply). Returns its addr.
func nbFixture(t *testing.T, respond func(req []byte) []byte) netip.AddrPort {
	t.Helper()
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if resp := respond(buf[:n]); resp != nil {
				_, _ = pc.WriteToUDP(resp, addr)
			}
		}
	}()
	return netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(pc.LocalAddr().(*net.UDPAddr).Port))
}

func TestNBNodeStatus_LiveFixture(t *testing.T) {
	addr := nbFixture(t, func(req []byte) []byte {
		if len(req) < 2 {
			return nil
		}
		id := binary.BigEndian.Uint16(req[:2])
		return nodeStatusResponse(id, []nbEntry{{name: nbName("DESKTOP-ABC"), suffix: 0x00, flags: 0x0400}})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if got := nbNodeStatus(ctx, addr); got != "DESKTOP-ABC" {
		t.Errorf("nbNodeStatus = %q, want %q", got, "DESKTOP-ABC")
	}
}

func TestNBNodeStatus_WrongTxnID(t *testing.T) {
	addr := nbFixture(t, func(req []byte) []byte {
		return nodeStatusResponse(0xFFFF, []nbEntry{{name: nbName("DESKTOP-ABC"), suffix: 0x00}})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if got := nbNodeStatus(ctx, addr); got != "" {
		t.Errorf("nbNodeStatus = %q, want %q for mismatched transaction ID", got, "")
	}
}

func TestNBNodeStatus_GarbageResponse(t *testing.T) {
	addr := nbFixture(t, func(req []byte) []byte {
		return []byte("garbage garbage garbage garbage garbage")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if got := nbNodeStatus(ctx, addr); got != "" {
		t.Errorf("nbNodeStatus = %q, want %q for garbage response", got, "")
	}
}

func TestNBNodeStatus_NoResponse(t *testing.T) {
	addr := nbFixture(t, func(req []byte) []byte { return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if got := nbNodeStatus(ctx, addr); got != "" {
		t.Errorf("nbNodeStatus = %q, want %q with no response", got, "")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("nbNodeStatus took %v, want bounded by ctx", elapsed)
	}
}

func TestNetBIOSName_Unreachable(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1 (RFC 5737): guaranteed unroutable.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if got := NetBIOSName(ctx, netip.MustParseAddr("192.0.2.1")); got != "" {
		t.Errorf("NetBIOSName = %q, want %q for unroutable ip", got, "")
	}
}

func TestNetBIOSName_CancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := NetBIOSName(ctx, netip.MustParseAddr("127.0.0.1")); got != "" {
		t.Errorf("NetBIOSName = %q, want %q with cancelled ctx", got, "")
	}
}

func TestReverseName_DocRange(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1 (RFC 5737): no PTR record exists.
	if got := ReverseName(netip.MustParseAddr("192.0.2.1")); got != "" {
		t.Errorf("ReverseName = %q, want %q for doc-range ip", got, "")
	}
}

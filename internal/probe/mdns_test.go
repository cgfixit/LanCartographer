package probe

import (
	"context"
	"net/netip"
	"testing"
)

// wantQuery is the byte-exact mDNS PTR query for 192.168.1.5 with
// transaction ID 0x1234: header (id, zero flags, QD=1, AN/NS/AR=0) followed
// by the reversed "5.1.168.192.in-addr.arpa" question, type PTR, class IN.
var wantQuery = []byte{
	0x12, 0x34, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x01, '5',
	0x01, '1',
	0x03, '1', '6', '8',
	0x03, '1', '9', '2',
	0x07, 'i', 'n', '-', 'a', 'd', 'd', 'r',
	0x04, 'a', 'r', 'p', 'a',
	0x00,
	0x00, 0x0c, // PTR
	0x00, 0x01, // IN
}

func TestBuildMDNSQuery_Fixture(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.5")
	got := buildMDNSQuery(0x1234, ip)
	if len(got) != len(wantQuery) {
		t.Fatalf("buildMDNSQuery len = %d, want %d\n got: %x", len(got), len(wantQuery), got)
	}
	for i := range wantQuery {
		if got[i] != wantQuery[i] {
			t.Fatalf("buildMDNSQuery[%d] = %02x, want %02x\n got: %x", i, got[i], wantQuery[i], got)
		}
	}
}

// mdnsResponse assembles a minimal response: one literal question for
// 5.1.168.192.in-addr.arpa plus the given raw answer records.
func mdnsResponse(id uint16, answers ...[]byte) []byte {
	var b []byte
	b = append(b, byte(id>>8), byte(id))
	b = append(b, 0x84, 0x00)                                // response + authoritative
	b = append(b, 0x00, 0x01)                                // QDCOUNT
	b = append(b, byte(len(answers)>>8), byte(len(answers))) // ANCOUNT
	b = append(b, 0x00, 0x00, 0x00, 0x00)                    // NSCOUNT, ARCOUNT
	b = append(b, wantQuery[12:]...)                         // question section
	for _, a := range answers {
		b = append(b, a...)
	}
	return b
}

// ptrAnswer builds an answer record: owner C0 0C (pointer into the question),
// type PTR, class IN, ttl 120s, then rdata.
func ptrAnswer(rdata []byte) []byte {
	b := []byte{0xc0, 0x0c, 0x00, 0x0c, 0x00, 0x01, 0x00, 0x00, 0x00, 0x78}
	rdlen := len(rdata)
	b = append(b, byte(rdlen>>8), byte(rdlen))
	return append(b, rdata...)
}

// literalOwnerResp builds a response whose single answer has the LITERAL
// owner name "printer.local" at offset 12 (no compression pointer), so a
// 0xC0 pointer in the rdata can target it — exercising rdata compression.
// QDCOUNT is 0: with a literal owner there is no shared question to point
// into, so the answer occupies the space right after the header.
func literalOwnerResp(rdata []byte) []byte {
	var b []byte
	b = append(b, byte(0x12), 0x34, 0x84, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00)
	b = append(b, 0x07, 'p', 'r', 'i', 'n', 't', 'e', 'r', 0x05, 'l', 'o', 'c', 'a', 'l', 0x00)
	b = append(b, 0x00, 0x0c, 0x00, 0x01, 0x00, 0x00, 0x00, 0x78)
	rdlen := len(rdata)
	b = append(b, byte(rdlen>>8), byte(rdlen))
	return append(b, rdata...)
}

func TestParsePTRName(t *testing.T) {
	target := netip.MustParseAddr("192.168.1.5")
	other := netip.MustParseAddr("192.168.1.99")

	litRdata := []byte{0x07, 'p', 'r', 'i', 'n', 't', 'e', 'r', 0x05, 'l', 'o', 'c', 'a', 'l', 0x00}
	truncated := ptrAnswer(litRdata)
	truncated = truncated[:len(truncated)-1] // rdlen now overruns the packet

	cases := []struct {
		name   string
		resp   []byte
		src    netip.Addr
		want   string
		wantOK bool
	}{
		{"simple literal rdata", mdnsResponse(0x1234, ptrAnswer(litRdata)), target, "printer", true},
		{"compressed rdata pointer", literalOwnerResp([]byte{0xc0, 0x0c}), target, "printer", true},
		{"wrong source rejected", mdnsResponse(0x1234, ptrAnswer(litRdata)), other, "", false},
		{"garbage header", []byte{0x01, 0x02, 0x03}, target, "", false},
		{"empty response", nil, target, "", false},
		{"no answers", mdnsResponse(0x1234), target, "", false},
		{"truncated rdata", mdnsResponse(0x1234, truncated), target, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parsePTRName(tc.resp, tc.src, target)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("parsePTRName = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParsePTRName_StripsLocalSuffix(t *testing.T) {
	target := netip.MustParseAddr("192.168.1.5")
	rdata := []byte{0x04, 'c', 'a', 's', 't', 0x05, 'l', 'o', 'c', 'a', 'l', 0x00}
	resp := mdnsResponse(0x1234, ptrAnswer(rdata))
	got, ok := parsePTRName(resp, target, target)
	if !ok || got != "cast" {
		t.Errorf("parsePTRName = %q, %v; want cast, true", got, ok)
	}
}

func TestParsePTRName_NonPTRAnswerIgnored(t *testing.T) {
	target := netip.MustParseAddr("192.168.1.5")
	// A-record-shaped answer (type 0x0001, rdlen 4) where a PTR was wanted.
	aAnswer := []byte{0xc0, 0x0c, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x78, 0x00, 0x04, 192, 168, 1, 5}
	resp := mdnsResponse(0x1234, aAnswer)
	if got, ok := parsePTRName(resp, target, target); ok || got != "" {
		t.Errorf("parsePTRName = %q, %v; want \"\", false for non-PTR answer", got, ok)
	}
}

func TestMDNSName_Unreachable(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1 (RFC 5737): no mDNS responder answers there,
	// so the lookup must time out to "" well inside its budget.
	if got := MDNSName(context.Background(), netip.MustParseAddr("192.0.2.1")); got != "" {
		t.Errorf("MDNSName = %q, want %q for unroutable ip", got, "")
	}
}

func TestMDNSName_CancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := MDNSName(ctx, netip.MustParseAddr("127.0.0.1")); got != "" {
		t.Errorf("MDNSName = %q, want %q with cancelled ctx", got, "")
	}
}

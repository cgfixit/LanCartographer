package winutil

import (
	"net/netip"
	"strings"
	"testing"
)

func TestSendARPLoopbackCacheMiss(t *testing.T) {
	mac, err := SendARP(netip.MustParseAddr("127.0.0.254"))
	if mac != [6]byte{} || err == nil || !strings.HasSuffix(err.Error(), ": no ARP entry") {
		t.Fatalf("loopback cache miss reported %x, %v", mac, err)
	}
}

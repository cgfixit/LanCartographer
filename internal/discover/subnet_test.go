package discover

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/local/lancartographer/internal/winutil"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
}

func TestHostsIn24(t *testing.T) {
	addrs := HostsIn(mustPrefix(t, "192.168.1.0/24"))
	if len(addrs) != 254 {
		t.Fatalf("HostsIn(/24) returned %d addrs, want 254", len(addrs))
	}
	if addrs[0] != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("first addr = %v, want 192.168.1.1", addrs[0])
	}
	if addrs[len(addrs)-1] != netip.MustParseAddr("192.168.1.254") {
		t.Errorf("last addr = %v, want 192.168.1.254", addrs[len(addrs)-1])
	}
	for i := 1; i < len(addrs); i++ {
		if addrs[i].Compare(addrs[i-1]) <= 0 {
			t.Fatalf("addrs not strictly ascending at %d: %v then %v", i, addrs[i-1], addrs[i])
		}
	}
	for _, a := range addrs {
		if a == netip.MustParseAddr("192.168.1.0") || a == netip.MustParseAddr("192.168.1.255") {
			t.Errorf("network/broadcast addr %v included", a)
		}
	}
}

func TestHostsIn32(t *testing.T) {
	addrs := HostsIn(mustPrefix(t, "10.0.0.7/32"))
	if len(addrs) != 1 || addrs[0] != netip.MustParseAddr("10.0.0.7") {
		t.Fatalf("HostsIn(/32) = %v, want [10.0.0.7]", addrs)
	}
}

func TestHostsIn31(t *testing.T) {
	addrs := HostsIn(mustPrefix(t, "10.0.0.4/31"))
	want := []netip.Addr{netip.MustParseAddr("10.0.0.4"), netip.MustParseAddr("10.0.0.5")}
	if len(addrs) != len(want) {
		t.Fatalf("HostsIn(/31) = %v, want %v", addrs, want)
	}
	for i := range want {
		if addrs[i] != want[i] {
			t.Errorf("addrs[%d] = %v, want %v", i, addrs[i], want[i])
		}
	}
}

// fakeAdapters installs a fake adapter list and returns a restore func.
func fakeAdapters(t *testing.T, aps []winutil.AdapterPrefix, err error) {
	t.Helper()
	orig := adapterPrefixes
	adapterPrefixes = func() ([]winutil.AdapterPrefix, error) { return aps, err }
	t.Cleanup(func() { adapterPrefixes = orig })
}

func ap(name, prefix string, ifType uint32, up bool) winutil.AdapterPrefix {
	return winutil.AdapterPrefix{
		Name:   name,
		Prefix: netip.MustParsePrefix(prefix).Masked(),
		IfType: ifType,
		Up:     up,
	}
}

func TestLocalSubnetsKeepsPhysicalUpIPv4(t *testing.T) {
	fakeAdapters(t, []winutil.AdapterPrefix{
		ap("Ethernet", "192.168.1.10/24", 6, true),
		ap("Wi-Fi", "10.0.0.5/24", 71, true),
	}, nil)
	infos, err := LocalSubnets(false)
	if err != nil {
		t.Fatalf("LocalSubnets: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d interfaces, want 2: %v", len(infos), infos)
	}
	if infos[0].Name != "Ethernet" || infos[0].Prefix != mustPrefix(t, "192.168.1.0/24") {
		t.Errorf("infos[0] = %+v", infos[0])
	}
	if infos[1].Name != "Wi-Fi" || infos[1].Prefix != mustPrefix(t, "10.0.0.0/24") {
		t.Errorf("infos[1] = %+v", infos[1])
	}
}

func TestLocalSubnetsDropsVirtualUnlessIncluded(t *testing.T) {
	aps := []winutil.AdapterPrefix{
		ap("Ethernet", "192.168.1.10/24", 6, true),
		ap("WSL", "172.20.0.1/20", 131, true),
	}
	fakeAdapters(t, aps, nil)
	infos, err := LocalSubnets(false)
	if err != nil {
		t.Fatalf("LocalSubnets: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != "Ethernet" {
		t.Fatalf("LocalSubnets(false) = %v, want only Ethernet", infos)
	}
	infos, err = LocalSubnets(true)
	if err != nil {
		t.Fatalf("LocalSubnets: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("LocalSubnets(true) = %v, want Ethernet + WSL", infos)
	}
}

func TestLocalSubnetsDropsDownLoopbackLinkLocal(t *testing.T) {
	fakeAdapters(t, []winutil.AdapterPrefix{
		ap("Ethernet", "192.168.1.10/24", 6, true),
		ap("Ethernet Down", "192.168.99.10/24", 6, false),
		ap("Loopback", "127.0.0.1/8", 24, true),
		ap("LinkLocal", "169.254.5.5/16", 6, true),
	}, nil)
	infos, err := LocalSubnets(false)
	if err != nil {
		t.Fatalf("LocalSubnets: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != "Ethernet" {
		t.Fatalf("LocalSubnets = %v, want only Ethernet", infos)
	}
}

func TestLocalSubnetsDropsIPv6(t *testing.T) {
	fakeAdapters(t, []winutil.AdapterPrefix{
		{Name: "Ethernet", Prefix: netip.MustParsePrefix("fe80::1/64"), IfType: 6, Up: true},
		ap("Ethernet", "192.168.1.10/24", 6, true),
	}, nil)
	infos, err := LocalSubnets(false)
	if err != nil {
		t.Fatalf("LocalSubnets: %v", err)
	}
	if len(infos) != 1 || !infos[0].Prefix.Addr().Is4() {
		t.Fatalf("LocalSubnets = %v, want only the IPv4 subnet", infos)
	}
}

func TestLocalSubnetsPropagatesError(t *testing.T) {
	fakeAdapters(t, nil, errors.New("boom"))
	if _, err := LocalSubnets(false); err == nil {
		t.Fatal("LocalSubnets with failing adapter source returned nil error")
	}
}

package maplayout

import (
	"fmt"
	"math"
	"net/netip"
	"testing"

	"github.com/local/lancartographer/internal/model"
)

func mustIP(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return a
}

func host(ip netip.Addr) model.Host {
	return model.Host{IP: ip, Alive: true}
}

func sampleHosts(t *testing.T, center netip.Addr) []model.Host {
	t.Helper()
	ips := []string{
		"192.168.1.10", "192.168.1.20", "192.168.1.30",
		"10.0.0.5", "10.0.0.20", "172.16.0.7",
	}
	hosts := make([]model.Host, 0, len(ips)+1)
	hosts = append(hosts, host(center))
	for _, s := range ips {
		hosts = append(hosts, host(mustIP(t, s)))
	}
	return hosts
}

func TestLayoutInputOrderIndependent(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	hosts := sampleHosts(t, center)

	reversed := make([]model.Host, len(hosts))
	for i, h := range hosts {
		reversed[len(hosts)-1-i] = h
	}

	a := Layout(hosts, center)
	b := Layout(reversed, center)
	if len(a) != len(b) {
		t.Fatalf("len differs: %d vs %d", len(a), len(b))
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			t.Fatalf("key %q missing in second layout", k)
		}
		if va != vb {
			t.Fatalf("key %q: %v != %v (input order leaked into output)", k, va, vb)
		}
	}
}

func TestLayoutCenterAtOrigin(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	m := Layout(sampleHosts(t, center), center)
	pt, ok := m[center.String()]
	if !ok {
		t.Fatalf("center key %q missing", center.String())
	}
	if pt != [2]float64{0, 0} {
		t.Fatalf("center at %v, want {0 0}", pt)
	}
}

func TestLayoutCenterSynthesizedWhenAbsent(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	hosts := sampleHosts(t, center)[1:] // drop the center host
	m := Layout(hosts, center)
	if pt := m[center.String()]; pt != [2]float64{0, 0} {
		t.Fatalf("synthesized center at %v, want {0 0}", pt)
	}
	if len(m) != len(hosts)+1 {
		t.Fatalf("len = %d, want %d", len(m), len(hosts)+1)
	}
}

func TestLayoutRingRadiusAndDistinctness(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	hosts := sampleHosts(t, center)
	m := Layout(hosts, center)

	seen := make(map[[2]float64]string, len(m))
	for _, h := range hosts {
		pt := m[h.IP.String()]
		if h.IP == center {
			continue
		}
		r := math.Hypot(pt[0], pt[1])
		if math.Abs(r-1.0) > 1e-9 {
			t.Fatalf("host %s radius = %v, want 1.0", h.IP, r)
		}
		if prev, dup := seen[pt]; dup {
			t.Fatalf("hosts %s and %s share point %v", prev, h.IP, pt)
		}
		seen[pt] = h.IP.String()
	}
}

func TestLayoutAngleOrderByLastOctet(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	hosts := []model.Host{
		host(center),
		host(mustIP(t, "10.0.0.30")),  // last octet 30
		host(mustIP(t, "10.0.0.10")),  // last octet 10
		host(mustIP(t, "192.168.1.20")), // last octet 20
	}
	m := Layout(hosts, center)

	angle := func(ip string) float64 {
		pt := m[ip]
		a := math.Atan2(pt[1], pt[0])
		if a < 0 {
			a += 2 * math.Pi
		}
		return a
	}
	a10, a20, a30 := angle("10.0.0.10"), angle("192.168.1.20"), angle("10.0.0.30")
	if !(a10 < a20 && a20 < a30) {
		t.Fatalf("angles not ordered by last octet: 10→%v 20→%v 30→%v", a10, a20, a30)
	}
	n := float64(3)
	if math.Abs(a10-0) > 1e-9 {
		t.Fatalf("first ring host angle = %v, want 0", a10)
	}
	if math.Abs(a20-2*math.Pi*1/n) > 1e-9 || math.Abs(a30-2*math.Pi*2/n) > 1e-9 {
		t.Fatalf("angles = %v, %v; want 2π/3 steps", a20, a30)
	}
}

func TestLayoutLastOctetTieBreaksByFullIP(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	lo := host(mustIP(t, "10.0.0.20"))
	hi := host(mustIP(t, "192.168.1.20")) // same last octet, higher full IP
	m := Layout([]model.Host{host(center), hi, lo}, center)

	angle := func(ip string) float64 {
		pt := m[ip]
		a := math.Atan2(pt[1], pt[0])
		if a < 0 {
			a += 2 * math.Pi
		}
		return a
	}
	if angle(lo.IP.String()) >= angle(hi.IP.String()) {
		t.Fatalf("tie-break wrong: %s angle %v >= %s angle %v",
			lo.IP, angle(lo.IP.String()), hi.IP, angle(hi.IP.String()))
	}
}

func TestLayoutDeterministicAcrossCalls(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	hosts := sampleHosts(t, center)
	first := Layout(hosts, center)
	for i := 0; i < 10; i++ {
		got := Layout(hosts, center)
		for k, v := range first {
			if got[k] != v {
				t.Fatalf("call %d: key %q = %v, want %v", i, k, got[k], v)
			}
		}
	}
}

func TestLayoutMoreThanRingCapacity(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	hosts := []model.Host{host(center)}
	for i := 0; i < 300; i++ {
		hosts = append(hosts, host(mustIP(t, fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256))))
	}
	m := Layout(hosts, center) // must not panic or hang
	if len(m) != len(hosts) {
		t.Fatalf("len = %d, want %d", len(m), len(hosts))
	}
	again := Layout(hosts, center)
	for k, v := range m {
		if again[k] != v {
			t.Fatalf("non-deterministic at key %q", k)
		}
	}
}

func TestLayoutEmptyAndCenterOnly(t *testing.T) {
	center := mustIP(t, "192.168.1.1")
	m := Layout(nil, center)
	if len(m) != 1 || m[center.String()] != [2]float64{0, 0} {
		t.Fatalf("empty input layout = %v, want just the center at origin", m)
	}
	m = Layout([]model.Host{host(center)}, center)
	if len(m) != 1 {
		t.Fatalf("center-only layout len = %d, want 1", len(m))
	}
}

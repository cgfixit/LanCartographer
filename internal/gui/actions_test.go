package gui

import (
	"net/netip"
	"testing"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
)

func host(ip string, ports ...model.Port) model.Host {
	return model.Host{IP: netip.MustParseAddr(ip), Alive: true, Ports: ports}
}

func TestBrowserURL(t *testing.T) {
	open := func(n int) model.Port { return model.Port{Number: n, State: model.PortOpen, Service: "svc"} }
	cases := []struct {
		name  string
		ports []model.Port
		want  string
	}{
		{"no ports", nil, ""},
		{"8443 open", []model.Port{open(8443)}, "https://10.0.0.1:8443"},
		{"only closed 80", []model.Port{{Number: 80, State: model.PortClosed}}, ""},
		{"80 open", []model.Port{open(80)}, "http://10.0.0.1"},
		{"8080 open", []model.Port{open(8080)}, "http://10.0.0.1:8080"},
		{"443 only", []model.Port{open(443)}, "https://10.0.0.1"},
		{"443 plus 80", []model.Port{open(443), open(80)}, "http://10.0.0.1"},
		{"closed 443 only", []model.Port{{Number: 443, State: model.PortClosed}}, ""},
		{"detected development web server", []model.Port{{Number: 3000, State: model.PortOpen, Service: "Web"}}, "http://10.0.0.1:3000"},
		{"unidentified custom port", []model.Port{open(3000)}, ""},
		{"closed custom web port", []model.Port{{Number: 3000, State: model.PortClosed, Service: "Web"}}, ""},
		{"invalid imported web port", []model.Port{{Number: 65536, State: model.PortOpen, Service: "Web"}}, ""},
		{"standard web preference", []model.Port{{Number: 3000, State: model.PortOpen, Service: "Web"}, open(443)}, "https://10.0.0.1"},
	}
	for _, c := range cases {
		if got := browserURL(host("10.0.0.1", c.ports...)); got != c.want {
			t.Errorf("%s: browserURL = %q, want %q", c.name, got, c.want)
		}
	}
	if got := browserURL(host("::1", model.Port{Number: 3000, State: model.PortOpen, Service: "Web"})); got != "http://[::1]:3000" {
		t.Fatalf("imported IPv6 web endpoint = %q", got)
	}
}

func TestThemeModeLabelNamesTheSwitchTarget(t *testing.T) {
	for mode, want := range map[string]string{"light": "Dark theme", "dark": "Light theme", "other": "Dark theme"} {
		if got := themeModeLabel(mode); got != want {
			t.Fatalf("themeModeLabel(%q) = %q, want %q", mode, got, want)
		}
	}
}

func TestDisplayHelpers(t *testing.T) {
	h := host("10.0.0.1",
		model.Port{Number: 443, State: model.PortOpen, Service: "Web"},
		model.Port{Number: 80, State: model.PortClosed},
		model.Port{Number: 18443, State: model.PortOpen, Service: "unknown"},
		model.Port{Number: 22, State: model.PortOpen, Service: "SSH"})
	if got := portsCell(h); got != "22 SSH, 443 Web, 18443" {
		t.Fatalf("portsCell = %q", got)
	}
	if got := portsCell(host("10.0.0.2")); got != "" {
		t.Fatalf("portsCell without open ports = %q", got)
	}
	for in, want := range map[string]string{"tcp": "TCP connection", "arp": "ARP cache", "icmp": "Ping (ICMP echo)", "": "—", "other": "other"} {
		if got := foundViaLabel(in); got != want {
			t.Fatalf("foundViaLabel(%q) = %q, want %q", in, got, want)
		}
	}
	if plural(1, "host") != "1 host" || plural(0, "host") != "0 hosts" || plural(4, "open port") != "4 open ports" {
		t.Fatal("plural mismatch")
	}
}

func TestCanWOL(t *testing.T) {
	mustParse := netip.MustParseAddr
	cases := []struct {
		mac  string
		want bool
	}{
		{"", false},
		{probe.MacNeedsElevation, false},
		{"not-a-mac", false},
		{"00:11:22:33:44", false}, // too short
		{"00:11:22:33:44:55", true},
	}
	for _, c := range cases {
		h := model.Host{IP: mustParse("10.0.0.2"), MAC: c.mac}
		if got := canWOL(h); got != c.want {
			t.Errorf("canWOL(%q) = %v, want %v", c.mac, got, c.want)
		}
	}
}

func TestBroadcastAddr(t *testing.T) {
	cases := map[string]string{
		"10.0.0.0/24":    "10.0.0.255:9",
		"10.0.0.0/22":    "10.0.3.255:9",
		"192.168.7.9/30": "192.168.7.11:9",
		"10.7.0.0/16":    "10.7.255.255:9",
	}
	for cidr, want := range cases {
		p := netip.MustParsePrefix(cidr)
		if got := broadcastAddr(p); got != want {
			t.Errorf("broadcastAddr(%s) = %q, want %q", cidr, got, want)
		}
	}
}

func TestSharesCell(t *testing.T) {
	h := host("10.0.0.1")
	if got := sharesCell(h); got != "" {
		t.Errorf("empty = %q", got)
	}
	h.Shares = []model.Share{{Remark: "shares: access denied"}}
	if got := sharesCell(h); got != "access denied" {
		t.Errorf("denial = %q", got)
	}
	h.Shares = []model.Share{{Name: "DATA", UNC: `\\10.0.0.1\DATA`}, {Name: "IPC$"}}
	if got := sharesCell(h); got != "2 shares (DATA)" {
		t.Errorf("two shares = %q", got)
	}
	if !isDenialShare(model.Share{Remark: "shares: access denied"}) {
		t.Error("denial share not detected")
	}
	if isDenialShare(model.Share{Name: "IPC$"}) {
		t.Error("IPC$ mistaken for denial")
	}
}

func TestStoreMergesEvents(t *testing.T) {
	s := newHostStore()
	p80 := model.Port{Number: 80, State: model.PortOpen, Service: "http"}
	p445 := model.Port{Number: 445, State: model.PortOpen, Service: "smb"}

	// Live port event on an unknown host creates a stub.
	if ip := s.apply(scan.Event{Kind: scan.EventPortFound, Host: model.Host{IP: netip.MustParseAddr("10.0.0.5"), Ports: []model.Port{p80}}}); ip != "10.0.0.5" {
		t.Errorf("port event ip = %q", ip)
	}
	if got := s.len(); got != 1 {
		t.Fatalf("len = %d, want 1", got)
	}

	// Canonical host event replaces the record wholesale — the engine
	// re-emits the full port/share set on the canonical record, so the
	// stub's accumulated observations are dropped by design.
	full := model.Host{
		IP:     netip.MustParseAddr("10.0.0.5"),
		Alive:  true,
		Name:   "NAS",
		MAC:    "00:11:22:33:44:55",
		Vendor: "Acme",
		Ports:  []model.Port{p80, p445},
		Shares: []model.Share{{Name: "DATA", UNC: `\\10.0.0.5\DATA`}},
	}
	s.apply(scan.Event{Kind: scan.EventHostFound, Host: full})
	got, ok := s.get("10.0.0.5")
	if !ok || got.Name != "NAS" || len(got.Ports) != 2 || len(got.Shares) != 1 {
		t.Errorf("canonical record wrong: %+v", got)
	}

	// Live share event after the canonical record merges in.
	s.apply(scan.Event{Kind: scan.EventShareFound, Host: model.Host{
		IP:     netip.MustParseAddr("10.0.0.5"),
		Shares: []model.Share{{Name: "IPC$"}},
	}})
	got, _ = s.get("10.0.0.5")
	if len(got.Shares) != 2 {
		t.Errorf("shares after merge = %d, want 2", len(got.Shares))
	}

	// Second canonical event (engine re-emit or rescan) replaces again.
	s.apply(scan.Event{Kind: scan.EventHostFound, Host: full})
	got, _ = s.get("10.0.0.5")
	if len(got.Shares) != 1 {
		t.Errorf("re-canonical should reset shares, got %d", len(got.Shares))
	}

	// Progress/done events carry no host and touch nothing.
	if ip := s.apply(scan.Event{Kind: scan.EventProgress, Progress: 0.5}); ip != "" {
		t.Errorf("progress ip = %q", ip)
	}
	if ip := s.apply(scan.Event{Kind: scan.EventDone, Progress: 1}); ip != "" {
		t.Errorf("done ip = %q", ip)
	}
}

func TestStoreSnapshotSortsAndCounts(t *testing.T) {
	s := newHostStore()
	s.apply(scan.Event{Kind: scan.EventHostFound, Host: host("10.0.0.20",
		model.Port{Number: 80, State: model.PortOpen}, model.Port{Number: 81, State: model.PortClosed})})
	s.apply(scan.Event{Kind: scan.EventHostFound, Host: host("10.0.0.3")})
	snap := s.snapshot()
	if len(snap) != 2 || snap[0].IP.String() != "10.0.0.3" || snap[1].IP.String() != "10.0.0.20" {
		t.Errorf("snapshot order = %+v", snap)
	}
	hosts, openPorts, shares := s.counts()
	if hosts != 2 || openPorts != 1 || shares != 0 {
		t.Errorf("counts = %d,%d,%d", hosts, openPorts, shares)
	}
	s.apply(scan.Event{Kind: scan.EventShareFound, Host: model.Host{
		IP:     netip.MustParseAddr("10.0.0.3"),
		Shares: []model.Share{{Remark: "shares: access denied"}},
	}})
	if _, _, shares := s.counts(); shares != 0 {
		t.Errorf("access-denied marker counted as %d shares", shares)
	}
	s.apply(scan.Event{Kind: scan.EventShareFound, Host: model.Host{
		IP:     netip.MustParseAddr("10.0.0.20"),
		Shares: []model.Share{{Name: "Public"}},
	}})
	if _, _, shares := s.counts(); shares != 1 {
		t.Errorf("discovered share count = %d, want 1", shares)
	}

	// Mutating a snapshot record must not touch the store.
	snap[0].Name = "MUTATED"
	if got, _ := s.get("10.0.0.3"); got.Name == "MUTATED" {
		t.Error("snapshot aliases store state")
	}

	// Favorites path: wholesale replace, duplicates deduped.
	s.set([]model.Host{host("192.168.1.50"), host("192.168.1.50")})
	if got := s.len(); got != 1 {
		t.Errorf("set dedupe len = %d", got)
	}
	if _, ok := s.get("10.0.0.20"); ok {
		t.Error("set should replace inventory")
	}
}

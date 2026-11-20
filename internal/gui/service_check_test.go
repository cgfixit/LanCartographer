//go:build cgo

package gui

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/authz"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/servicecheck"
)

func TestToolsMenuOffersCheckServiceOnEveryPlatform(t *testing.T) {
	u := newTestUI(t)
	var tools []string
	for _, m := range u.mainMenu().Items {
		if m.Label == "Tools" {
			for _, item := range m.Items {
				tools = append(tools, item.Label)
			}
		}
	}
	want := []string{"Check Service…"}
	if runtime.GOOS == "darwin" {
		want = append(want, "Internet Quality…", "Connections…", "Route & DNS…")
	}
	if strings.Join(tools, "|") != strings.Join(want, "|") {
		t.Fatalf("Tools menu = %v, want %v", tools, want)
	}
}

func TestServicePrefillPrefersWebPortsAndSendsNothing(t *testing.T) {
	u := newTestUI(t)
	h := model.Host{IP: netip.MustParseAddr("192.168.1.20"), Ports: []model.Port{
		{Number: 22, State: model.PortOpen}, {Number: 80, State: model.PortOpen},
		{Number: 443, State: model.PortClosed}, {Number: 8443, State: model.PortOpen},
	}}
	u.showServiceCheck(servicePrefillFor(h))
	v := u.serviceView
	if v == nil {
		t.Fatal("window did not open")
	}
	t.Cleanup(v.win.Close)
	if v.host.Text != "192.168.1.20" || v.port.Text != "8443" || v.protocol.Selected != "HTTPS" {
		t.Fatalf("prefill = %q %q %q", v.host.Text, v.port.Text, v.protocol.Selected)
	}
	if v.job != nil || v.last != nil || v.address.Selected != "192.168.1.20" || !v.resolve.Disabled() {
		t.Fatalf("prefill started work or left controls wrong: job=%v last=%v addr=%q", v.job, v.last, v.address.Selected)
	}
	if p := servicePrefillFor(model.Host{IP: h.IP}); p.port != 0 {
		t.Fatalf("a host without open ports prefilled port %d", p.port)
	}
	web := servicePrefillFor(model.Host{IP: h.IP, Ports: []model.Port{
		{Number: 22, State: model.PortOpen}, {Number: 18080, State: model.PortOpen, Service: "Web"},
	}})
	if web.port != 18080 || web.protocol != servicecheck.HTTP {
		t.Fatalf("a scanned web port prefilled %d/%s", web.port, web.protocol)
	}
}

func TestServicePortSuggestsProtocolUntilUserChooses(t *testing.T) {
	u := newTestUI(t)
	u.showServiceCheck(nil)
	v := u.serviceView
	t.Cleanup(v.win.Close)
	v.port.SetText("8080")
	if v.protocol.Selected != "HTTP" {
		t.Fatalf("8080 suggested %q", v.protocol.Selected)
	}
	v.protocol.SetSelected("TLS")
	v.port.SetText("443")
	if v.protocol.Selected != "TLS" {
		t.Fatalf("a port edit replaced the chosen protocol with %q", v.protocol.Selected)
	}
	v.apply(&servicePrefill{host: "192.168.1.9", port: 443})
	if v.protocol.Selected != "HTTPS" {
		t.Fatalf("a new prefill should suggest again, got %q", v.protocol.Selected)
	}
}

func TestServiceConfirmTextNamesEndpointScopeAndNotice(t *testing.T) {
	local := []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}
	target := servicecheck.Target{Host: "nas.home.arpa", Addr: netip.MustParseAddr("192.168.1.20"), Port: 443, Protocol: servicecheck.HTTPS}
	text := serviceConfirmText(target, local)
	for _, want := range []string{"192.168.1.20 port 443 as nas.home.arpa", "verified TLS handshake", "one HTTP GET /", "Redirects are not followed", authz.Notice} {
		if !strings.Contains(text, want) {
			t.Fatalf("confirmation lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Caution") {
		t.Fatalf("a local private address should not carry a caution:\n%s", text)
	}
	for addr, want := range map[string]string{
		"10.9.9.9":    "outside this computer's local subnets",
		"203.0.113.7": "public address",
		"2001:db8::1": "public IPv6",
		"fd00::1":     "private IPv6",
		"127.0.0.1":   "loopback",
		"169.254.1.9": "link-local address",
		"100.64.1.1":  "carrier-grade NAT",
	} {
		target.Addr = netip.MustParseAddr(addr)
		if got := serviceConfirmText(target, local); !strings.Contains(got, want) {
			t.Errorf("%s: confirmation lacks %q:\n%s", addr, want, got)
		}
	}
}

func TestServiceHostNameNeedsResolvedAddress(t *testing.T) {
	u := newTestUI(t)
	u.showServiceCheck(&servicePrefill{host: "nas.home.arpa", port: 443})
	v := u.serviceView
	t.Cleanup(v.win.Close)
	if v.resolve.Disabled() {
		t.Fatal("Resolve should be available for a host name")
	}
	if _, err := v.serviceTarget(); err == nil || !strings.Contains(err.Error(), "Resolve") {
		t.Fatalf("unresolved name produced a target: %v", err)
	}
	// Simulate a completed lookup, then choose an address.
	v.resolved = []netip.Addr{netip.MustParseAddr("192.168.1.20"), netip.MustParseAddr("192.168.1.21")}
	v.resolvedHost = "nas.home.arpa"
	v.address.Options = []string{"192.168.1.20", "192.168.1.21"}
	v.address.SetSelected("192.168.1.21")
	got, err := v.serviceTarget()
	if err != nil || got.Addr != netip.MustParseAddr("192.168.1.21") || got.Host != "nas.home.arpa" {
		t.Fatalf("target = %+v, %v", got, err)
	}
	// An address that the lookup did not return is refused.
	v.address.Selected = "10.0.0.1"
	if _, err := v.serviceTarget(); err == nil {
		t.Fatal("an address outside the resolver answer was accepted")
	}
	// Editing the host discards the old answer.
	v.host.SetText("other.home.arpa")
	if v.resolved != nil || v.address.Selected != "" {
		t.Fatalf("stale resolution survived a host edit: %v %q", v.resolved, v.address.Selected)
	}
}

// jobFinished returns a channel closed after the next job's UI callback.
func jobFinished(v *serviceView) <-chan struct{} {
	done := make(chan struct{})
	v.afterJob = func() { close(done) }
	return done
}

func wait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}
}

func TestServiceCheckRunsAgainstOwnedFixture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<title>Owned fixture</title>")
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	u := newTestUI(t)
	u.showServiceCheck(&servicePrefill{host: "127.0.0.1", port: port})
	v := u.serviceView
	t.Cleanup(v.win.Close)
	v.protocol.SetSelected("HTTP")
	target, err := v.serviceTarget()
	if err != nil {
		t.Fatal(err)
	}
	finished := jobFinished(v)
	u.runServiceCheck(v, target, nil)
	wait(t, finished)
	if v.last.Outcome() != servicecheck.Passed || !strings.HasPrefix(v.status.Text, "Service reachable") {
		t.Fatalf("status %q, result %+v", v.status.Text, v.last.Stages)
	}
	if len(v.stages.Objects) != 2 || v.copyBtn.Disabled() || !strings.Contains(v.copyText(), "Owned fixture") {
		t.Fatalf("rendered %d stage rows, copy disabled=%v", len(v.stages.Objects), v.copyBtn.Disabled())
	}
}

func TestServiceCheckCancelDropsLateResult(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	u := newTestUI(t)
	u.showServiceCheck(&servicePrefill{host: "127.0.0.1", port: l.Addr().(*net.TCPAddr).Port})
	v := u.serviceView
	t.Cleanup(v.win.Close)
	v.protocol.SetSelected("TLS")
	target, _ := v.serviceTarget()
	finished := jobFinished(v)
	u.runServiceCheck(v, target, nil)
	if v.job == nil || v.stop.Disabled() || !v.check.Disabled() {
		t.Fatal("running check should enable only Stop")
	}
	u.cancelServiceJob(v)
	wait(t, finished)
	if v.last != nil || v.status.Text != "Cancelled" || v.check.Disabled() {
		t.Fatalf("after cancel: last=%v status=%q", v.last, v.status.Text)
	}
}

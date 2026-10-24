package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/local/lancartographer/internal/discover"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
)

// fakeDeps returns a deps seam with one local subnet and a scanRun that
// emits the given hosts (plus EventDone) and returns nil.
func fakeDeps(local []string, hosts ...model.Host) deps {
	var infos []discover.InterfaceInfo
	for _, l := range local {
		infos = append(infos, discover.InterfaceInfo{Name: "fake0", Prefix: netip.MustParsePrefix(l)})
	}
	return deps{
		localSubnets: func(bool) ([]discover.InterfaceInfo, error) { return infos, nil },
		scanRun: func(_ context.Context, target netip.Prefix, _ probe.Options, out chan<- scan.Event) error {
			for _, h := range hosts {
				if target.Contains(h.IP) {
					out <- scan.Event{Kind: scan.EventHostFound, Host: h}
				}
			}
			out <- scan.Event{Kind: scan.EventDone, Progress: 1}
			return nil
		},
		exportToFile: func(path string, hosts []model.Host, format string) error {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			defer f.Close()
			return json.NewEncoder(f).Encode(hosts)
		},
		needsConfirmation: func(target netip.Prefix, local []netip.Prefix) bool {
			for _, l := range local {
				if l.Bits() <= target.Bits() && l.Contains(target.Addr()) {
					return false
				}
			}
			return true
		},
	}
}

func runCLI(d deps, args ...string) (code int, stdout, stderr string) {
	var outBuf, errBuf bytes.Buffer
	code = run(args, &outBuf, &errBuf, d)
	return code, outBuf.String(), errBuf.String()
}

func TestUnknownFlagExits2(t *testing.T) {
	code, _, stderr := runCLI(fakeDeps([]string{"192.168.1.0/24"}), "--bogus")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, stderr)
	}
}

func TestBadCIDRExits2(t *testing.T) {
	code, _, stderr := runCLI(fakeDeps([]string{"192.168.1.0/24"}), "--range", "not-a-cidr")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "not-a-cidr") {
		t.Fatalf("stderr %q does not name the bad range", stderr)
	}
}

func TestBadPortsExits2(t *testing.T) {
	code, _, _ := runCLI(fakeDeps([]string{"192.168.1.0/24"}), "--ports", "abc")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestNoLocalInterfacesExits3(t *testing.T) {
	d := fakeDeps(nil) // no interfaces, no --range
	code, _, stderr := runCLI(d)
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	if stderr == "" {
		t.Fatal("stderr should explain the missing interfaces")
	}
}

func TestConfirmationRefusalExits2(t *testing.T) {
	d := fakeDeps([]string{"192.168.1.0/24"})
	code, _, stderr := runCLI(d, "--range", "203.0.113.0/24")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "18 U.S.C. § 1030") {
		t.Fatalf("stderr %q lacks the authorization notice", stderr)
	}
	if !strings.Contains(stderr, "--i-am-authorized") {
		t.Fatalf("stderr %q lacks the override flag name", stderr)
	}
}

func TestConfirmationOverrideProceeds(t *testing.T) {
	h := model.Host{IP: netip.MustParseAddr("203.0.113.7"), Alive: true,
		Ports: []model.Port{{Number: 445, State: model.PortOpen, Service: "smb"}}}
	d := fakeDeps([]string{"192.168.1.0/24"}, h)
	code, stdout, _ := runCLI(d, "--range", "203.0.113.0/24", "--i-am-authorized")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, "203.0.113.7") || !strings.Contains(stdout, "445 open smb") {
		t.Fatalf("stdout missing host row:\n%s", stdout)
	}
}

func TestOmittedRangeScansLocalSubnets(t *testing.T) {
	h1 := model.Host{IP: netip.MustParseAddr("192.168.1.10"), Alive: true}
	h2 := model.Host{IP: netip.MustParseAddr("10.0.0.5"), Alive: true}
	d := fakeDeps([]string{"192.168.1.0/24", "10.0.0.0/24"}, h1, h2)
	// Local subnets are contained in local, so the fake gate passes both.
	code, stdout, _ := runCLI(d)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	i10 := strings.Index(stdout, "192.168.1.10")
	i5 := strings.Index(stdout, "10.0.0.5")
	if i10 < 0 || i5 < 0 {
		t.Fatalf("stdout missing merged hosts:\n%s", stdout)
	}
	// Sorted by IP ascending: 10.0.0.5 before 192.168.1.10.
	if i5 > i10 {
		t.Fatalf("rows not sorted by IP ascending:\n%s", stdout)
	}
}

func TestSharesCellFormatting(t *testing.T) {
	denied := model.Host{IP: netip.MustParseAddr("192.168.1.20"), Alive: true,
		Shares: []model.Share{{Remark: "shares: access denied"}}}
	named := model.Host{IP: netip.MustParseAddr("192.168.1.21"), Alive: true,
		Shares: []model.Share{{Name: "DATA", Type: "disk", Remark: "Main share"}, {Name: "IPC$", Type: "ipc"}}}
	d := fakeDeps([]string{"192.168.1.0/24"}, denied, named)
	code, stdout, _ := runCLI(d, "--range", "192.168.1.0/24")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, "shares: access denied") {
		t.Fatalf("stdout missing denial cell:\n%s", stdout)
	}
	if !strings.Contains(stdout, "DATA(disk):Main share, IPC$(ipc)") {
		t.Fatalf("stdout missing named-share cell:\n%s", stdout)
	}
}

func TestJSONExportRoundTrip(t *testing.T) {
	h := model.Host{IP: netip.MustParseAddr("192.168.1.30"), Alive: true, Name: "box"}
	d := fakeDeps([]string{"192.168.1.0/24"}, h)
	path := filepath.Join(t.TempDir(), "out.json")
	code, _, stderr := runCLI(d, "--range", "192.168.1.0/24", "--json", path)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var hosts []model.Host
	if err := json.Unmarshal(data, &hosts); err != nil {
		t.Fatalf("json not parseable: %v", err)
	}
	if len(hosts) != 1 || hosts[0].IP != h.IP {
		t.Fatalf("round trip mismatch: %+v", hosts)
	}
}

func TestTableSanitizesControlsWithoutChangingExport(t *testing.T) {
	raw := "café東京\x1b[2J\r\n\t\x00\x7f\u0085\u009b"
	h := model.Host{
		IP: netip.MustParseAddr("192.168.1.30"), Name: raw, MAC: raw, Vendor: raw,
		Ports:  []model.Port{{Number: 445, State: model.PortOpen, Service: raw}},
		Shares: []model.Share{{Name: raw, Type: raw, Remark: raw}},
	}
	path := filepath.Join(t.TempDir(), "out.json")
	code, stdout, stderr := runCLI(fakeDeps([]string{"192.168.1.0/24"}, h), "--json", path)
	if code != 0 {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	for _, r := range stdout {
		if unicode.IsControl(r) && r != '\n' {
			t.Fatalf("table contains control %U: %q", r, stdout)
		}
	}
	if strings.Count(stdout, "\n") != 2 {
		t.Fatalf("expected header and one host row: %q", stdout)
	}
	if got := strings.Count(stdout, "café東京�[2J�������"); got != 7 {
		t.Fatalf("expected seven sanitized values preserving Unicode, got %d: %q", got, stdout)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var hosts []model.Host
	if err := json.Unmarshal(data, &hosts); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hosts, []model.Host{h}) {
		t.Fatalf("display changed exported host: %+v", hosts)
	}
}

func TestJSONExportFailureExits2(t *testing.T) {
	d := fakeDeps([]string{"192.168.1.0/24"})
	d.exportToFile = func(string, []model.Host, string) error { return fmt.Errorf("disk full") }
	code, _, stderr := runCLI(d, "--range", "192.168.1.0/24", "--json", filepath.Join(t.TempDir(), "x.json"))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "disk full") {
		t.Fatalf("stderr %q lacks the export error", stderr)
	}
}

// httpFixture starts a listener that answers every connection with a
// minimal HTTP/1.0 response. Binding failure is tolerated when tolerateBusy
// is set (the port may already be served — liveness dials still succeed).
func httpFixture(t *testing.T, addr string, tolerateBusy bool) {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if tolerateBusy {
			return
		}
		t.Fatalf("listen %s: %v", addr, err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 512)
				c.SetDeadline(time.Now().Add(2 * time.Second))
				if _, err := c.Read(buf); err != nil {
					return
				}
				fmt.Fprintf(c, "HTTP/1.0 200 OK\r\nContent-Length: 0\r\n\r\n")
			}(conn)
		}
	}()
}

// TestLoopbackEndToEnd drives the REAL engine via Run on loopback. The
// liveness probe dials fixed DiscoveryPorts (8080 is one), so a second
// fixture on 8080 keeps 127.0.0.1 alive while the scan probes 18080.
func TestLoopbackEndToEnd(t *testing.T) {
	httpFixture(t, "127.0.0.1:18080", false)
	httpFixture(t, "127.0.0.1:8080", true)

	var outBuf, errBuf bytes.Buffer
	code := Run([]string{
		"--range", "127.0.0.1/32",
		"--ports", "18080",
		"--timeout", "200ms",
		"--i-am-authorized",
	}, &outBuf, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
	}
	stdout := outBuf.String()
	if !strings.Contains(stdout, "127.0.0.1") {
		t.Fatalf("stdout missing host IP:\n%s", stdout)
	}
	if !strings.Contains(strings.ToLower(stdout), "18080 open web") {
		t.Fatalf("stdout missing open web port:\n%s", stdout)
	}
}

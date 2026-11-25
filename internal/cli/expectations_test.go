package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
)

func writeExpectationFile(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expectations.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExpectationsReport(t *testing.T) {
	d := fakeDeps([]string{"192.168.1.0/24"}, model.Host{
		IP: netip.MustParseAddr("192.168.1.5"), Alive: true,
		Ports: []model.Port{
			{Number: 22, State: model.PortClosed},
			{Number: 23, State: model.PortTimeout},
			{Number: 80, State: model.PortClosed},
			{Number: 80, State: model.PortOpen},
			{Number: 443, State: model.PortOpen},
		},
	})
	_, baseline, _ := runCLI(d, "--json", "-")
	for _, tc := range []struct {
		name string
		body string
		code int
		want []string
	}{
		{
			name: "met with an implicit range",
			body: `{"version":1,"hosts":[{"ip":"192.168.1.5","open":[443],"closed":[22]},{"ip":"192.168.1.6","absent":true}]}`,
			code: 0,
			want: []string{"443 open: MET (scan reported open)", "22 closed: MET (scan reported closed)", "absent: MET (host not observed)"},
		},
		{
			name: "mismatch and unconfirmed",
			body: `{"version":1,"hosts":[{"ip":"192.168.1.5","closed":[23,443,80]},{"ip":"192.168.1.6","closed":[22]}]}`,
			code: 4,
			want: []string{"23 closed: NOT MET (scan reported timeout)", "443 closed: NOT MET (scan reported open)", "80 closed: NOT MET (scan reported conflicting observations)", "22 closed: NOT MET (scan reported no port result)"},
		},
		{
			name: "unexpected host",
			body: `{"version":1,"hosts":[{"ip":"192.168.1.5","absent":true}]}`,
			code: 4,
			want: []string{"absent: NOT MET (host observed)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeExpectationFile(t, tc.body)
			code, stdout, stderr := runCLI(d, "--expect", path, "--json", "-")
			if code != tc.code || stdout != baseline || !json.Valid([]byte(stdout)) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want exit %d and baseline JSON", code, stdout, stderr, tc.code)
			}
			for _, msg := range tc.want {
				if !strings.Contains(stderr, msg) {
					t.Errorf("stderr %q is missing %q", stderr, msg)
				}
			}
		})
	}
}

func TestExpectationsRefuseInvalidFileBeforeScan(t *testing.T) {
	const valid = `{"version":1,"hosts":[{"ip":"192.168.1.5","open":[443]}]}`
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"malformed", "{", "unexpected end"},
		{"trailing value", valid + "{}", "after top-level value"},
		{"unknown field", strings.Replace(valid, "open", "unexpected", 1), "unknown or duplicate field"},
		{"duplicate field", strings.Replace(valid, `"open"`, `"closed":[443],"closed"`, 1), "unknown or duplicate field"},
		{"null field", strings.Replace(valid, "[443]", "null", 1), "must not be null"},
		{"wrong version", strings.Replace(valid, ":1", ":2", 1), "expected version 1"},
		{"empty file of rules", `{"version":1,"hosts":[]}`, "at least one"},
		{"oversized", valid + strings.Repeat(" ", maxExpectationBytes), "file exceeds"},
		{"other subnet", strings.Replace(valid, "192.168.1.5", "192.168.2.5", 1), "outside the usable"},
		{"network address", strings.Replace(valid, "192.168.1.5", "192.168.1.0", 1), "outside the usable"},
		{"broadcast address", strings.Replace(valid, "192.168.1.5", "192.168.1.255", 1), "outside the usable"},
		{"ipv6", strings.Replace(valid, "192.168.1.5", "::1", 1), "outside the usable"},
		{"unselected port", strings.Replace(valid, "443", "5432", 1), "outside the scan port list"},
		{"repeated host", `{"version":1,"hosts":[{"ip":"192.168.1.5","absent":true},{"ip":"192.168.1.5","absent":true}]}`, "listed more than once"},
		{"contradictory port", strings.Replace(valid, `"open"`, `"closed":[443],"open"`, 1), "listed more than once"},
		{"absence with a port", strings.Replace(valid, `"open"`, `"absent":true,"open"`, 1), "either absence or port states"},
		{"no check", strings.Replace(valid, "[443]", "[]", 1), "either absence or port states"},
		{"too many checks", fmt.Sprintf(`{"version":1,"hosts":[{"ip":"192.168.1.5","open":[%s443]}]}`, strings.Repeat("443,", maxExpectationChecks)), "exceeds 4096 checks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fakeDeps([]string{"192.168.1.0/24"})
			d.scanRun = func(context.Context, netip.Prefix, probe.Options, chan<- scan.Event) error {
				t.Fatal("invalid expectations started a scan")
				return nil
			}
			code, stdout, stderr := runCLI(d, "--ports", "443", "--expect", writeExpectationFile(t, tc.body), "--json", "-")
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit=%d stdout=%q stderr=%q, want exit 2 and %q", code, stdout, stderr, tc.want)
			}
		})
	}
}

func TestExpectationsRequireAuthorizedCompleteScan(t *testing.T) {
	const body = `{"version":1,"hosts":[{"ip":"127.0.0.1","absent":true}]}`
	path := writeExpectationFile(t, body)
	args := []string{"--range", "127.0.0.1/32", "--expect", path}
	for _, tc := range []struct {
		name string
		args []string
		err  error
		want string
	}{
		{"refused", args, nil, "requires confirmation"},
		{"cancelled", append(args, "--i-am-authorized"), context.Canceled, "context canceled"},
		{"no completion event", append(args, "--i-am-authorized"), nil, "without a completion event"},
		{"same input and output", append(args, "--i-am-authorized", "--json", path), nil, "different files"},
		{"explicit empty path", []string{"--expect", ""}, nil, "must not be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fakeDeps(nil)
			scanned := false
			d.scanRun = func(context.Context, netip.Prefix, probe.Options, chan<- scan.Event) error {
				scanned = true
				return tc.err
			}
			code, stdout, stderr := runCLI(d, tc.args...)
			wantScan := tc.name == "cancelled" || tc.name == "no completion event"
			if code != 2 || stdout != "" || strings.Contains(stderr, " MET (") ||
				!strings.Contains(stderr, tc.want) || scanned != wantScan {
				t.Fatalf("exit=%d scanned=%t stdout=%q stderr=%q, want exit 2, scanned=%t and %q", code, scanned, stdout, stderr, wantScan, tc.want)
			}
		})
	}
}

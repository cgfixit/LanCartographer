//go:build windows

package gui

import (
	"net/netip"
	"slices"
	"testing"
)

func TestExplorerPath(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.5")
	if got := explorerPath(ip, ""); got != `\\10.0.0.5` {
		t.Errorf("host path = %q", got)
	}
	if got := explorerPath(ip, "DATA"); got != `\\10.0.0.5\DATA` {
		t.Errorf("share path = %q", got)
	}
}

func TestArgvBuilders(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.9")
	want := []string{"cmd", "/c", "start", "", "cmd", "/c", "ping", "-n", "4", "10.0.0.9"}
	if got := pingArgv(ip); !slices.Equal(got, want) {
		t.Errorf("pingArgv = %q", got)
	}
	want = []string{"mstsc", "/v:10.0.0.9"}
	if got := rdpArgv(ip); !slices.Equal(got, want) {
		t.Errorf("rdpArgv = %q", got)
	}
	want = []string{"cmd", "/c", "start", "", "http://10.0.0.9"}
	if got := browserArgv(ip, "http"); !slices.Equal(got, want) {
		t.Errorf("browserArgv http = %q", got)
	}
	if got := browserArgv(ip, ""); got != nil {
		t.Errorf("browserArgv empty scheme = %q, want nil", got)
	}
	want = []string{"explorer", `\\10.0.0.9\DATA`}
	if got := explorerArgv(explorerPath(ip, "DATA")); !slices.Equal(got, want) {
		t.Errorf("explorerArgv = %q", got)
	}
}

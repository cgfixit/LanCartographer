//go:build darwin

package gui

import (
	"net/netip"
	"slices"
	"testing"
)

func TestExplorerPath(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.5")
	cases := []struct {
		name  string
		share string
		want  string
	}{
		{"host only", "", "smb://10.0.0.5"},
		{"share", "DATA", "smb://10.0.0.5/DATA"},
	}
	for _, c := range cases {
		if got := explorerPath(ip, c.share); got != c.want {
			t.Errorf("%s: explorerPath = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestArgvBuilders(t *testing.T) {
	ip := netip.MustParseAddr("10.0.0.9")
	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{"ping", pingArgv(ip), []string{"osascript", "-e", `tell application "Terminal" to do script "ping -c 4 10.0.0.9"`}},
		{"rdp unavailable", rdpArgv(ip), nil},
		{"browser http", browserArgv(ip, "http"), []string{"open", "http://10.0.0.9"}},
		{"browser https", browserArgv(ip, "https"), []string{"open", "https://10.0.0.9"}},
		{"browser empty scheme", browserArgv(ip, ""), nil},
		{"explorer host", explorerArgv(explorerPath(ip, "")), []string{"open", "smb://10.0.0.9"}},
		{"explorer share", explorerArgv(explorerPath(ip, "DATA")), []string{"open", "smb://10.0.0.9/DATA"}},
	}
	for _, c := range cases {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("%s: argv = %q, want %q", c.name, c.got, c.want)
		}
	}
}

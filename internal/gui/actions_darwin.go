//go:build darwin

package gui

import (
	"net/netip"
	"net/url"
)

const fileManagerName = "Finder"

// explorerPath builds the smb:// URL for a host (smb://ip) or one of its
// shares (smb://ip/share), opened via the "open" command. A share whose
// Name is empty (the synthetic access-denied record) yields just the host
// URL.
func explorerPath(ip netip.Addr, shareName string) string {
	if shareName == "" {
		return "smb://" + ip.String()
	}
	return "smb://" + ip.String() + "/" + url.PathEscape(shareName)
}

// pingArgv builds the argv that opens a visible Terminal window running
// four pings against ip. ARGV LIST ONLY — never a shell string; osascript
// receives the AppleScript as one -e argument.
func pingArgv(ip netip.Addr) []string {
	return []string{"osascript", "-e", `tell application "Terminal" to do script "ping -c 4 ` + ip.String() + `"`}
}

// rdpArgv returns nil: no built-in RDP client exists on macOS, so the RDP
// action is unavailable and callers must hide it.
func rdpArgv(netip.Addr) []string {
	return nil
}

// browserArgv builds the argv that opens the host's web interface in the
// default browser via "open". target must come from browserURL (""
// means no action and returns nil).
func browserArgv(target string) []string {
	if target == "" {
		return nil
	}
	return []string{"open", target}
}

// explorerArgv builds the argv that opens a host or share smb:// URL in
// Finder via "open".
func explorerArgv(path string) []string {
	return []string{"open", path}
}

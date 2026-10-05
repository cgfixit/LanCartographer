//go:build windows

package gui

import "net/netip"

// explorerPath builds the UNC path for a host (\\ip) or one of its shares
// (\\ip\share). A share whose Name is empty (the synthetic access-denied
// record) yields just the host path.
func explorerPath(ip netip.Addr, shareName string) string {
	if shareName == "" {
		return `\\` + ip.String()
	}
	return `\\` + ip.String() + `\` + shareName
}

// pingArgv builds the argv that opens a visible console window running
// four pings against ip. ARGV LIST ONLY — never a shell string.
func pingArgv(ip netip.Addr) []string {
	return []string{"cmd", "/c", "start", "", "cmd", "/c", "ping", "-n", "4", ip.String()}
}

// rdpArgv builds the argv for an RDP session to ip.
func rdpArgv(ip netip.Addr) []string {
	return []string{"mstsc", "/v:" + ip.String()}
}

// browserArgv builds the argv that opens the host's web interface in the
// default browser. scheme must come from browserScheme ("" means no action
// and returns nil).
func browserArgv(ip netip.Addr, scheme string) []string {
	if scheme == "" {
		return nil
	}
	return []string{"cmd", "/c", "start", "", scheme + "://" + ip.String()}
}

// explorerArgv builds the argv that opens a host or share UNC in Explorer.
func explorerArgv(path string) []string {
	return []string{"explorer", path}
}

//go:build windows

package gui

import "net/netip"

const fileManagerName = "Explorer"

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
// four pings. Only a canonical IPv4 literal can enter the cmd command line.
func pingArgv(ip netip.Addr) []string {
	if !ip.Is4() {
		return nil
	}
	return []string{"cmd", "/c", "start", "", "cmd", "/c", "ping", "-n", "4", ip.String()}
}

// rdpArgv builds the argv for an RDP session to ip.
func rdpArgv(ip netip.Addr) []string {
	return []string{"mstsc", "/v:" + ip.String()}
}

// browserArgv builds the argv that opens the host's web interface in the
// default browser. target must come from browserURL ("" means no action
// and returns nil).
func browserArgv(target string) []string {
	if target == "" {
		return nil
	}
	return []string{"cmd", "/c", "start", "", target}
}

// explorerArgv builds the argv that opens a host or share UNC in Explorer.
func explorerArgv(path string) []string {
	return []string{"explorer", path}
}

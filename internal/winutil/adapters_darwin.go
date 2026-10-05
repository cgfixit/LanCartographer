//go:build darwin

package winutil

import (
	"net"
	"net/netip"
	"strings"
)

// darwinVirtualPrefixes names the interface-name prefixes macOS creates for
// its own plumbing (VPN tunnels, link-local wireless, tunnels, bridges, and
// hypervisor interfaces). They never carry a scannable LAN prefix, so
// AdapterPrefixes skips them by name.
var darwinVirtualPrefixes = []string{
	"utun",   // VPN / iCloud Private Relay tunnels
	"llw",    // low-latency WLAN (Apple Wireless Direct Link sibling)
	"gif",    // generic tunnel
	"bridge", // VM/bridge interfaces
	"vbox",   // VirtualBox host-only
	"vmware", // VMware host-only
	"awdl",   // Apple Wireless Direct Link (AirDrop)
	"stf",    // 6to4 tunnel
	"anpi",   // Apple Network Privacy Interface
}

// darwinIfTypeEthernet is the IfType reported for every adapter on darwin.
// The stdlib net.Interface does not expose the BSD IFT_* type, so the v1
// policy is to report 6 (IF_TYPE_ETHERNET_CSMACD, the same value physical
// adapters carry on Windows) for all of them; the virtual-name skip above
// matters more for scan correctness than a precise per-adapter type.
const darwinIfTypeEthernet = 6

// AdapterPrefixes enumerates every IPv4 unicast address on non-loopback, up,
// non-virtual interfaces via the stdlib net.Interfaces and reduces each one
// to its masked on-link prefix.
func AdapterPrefixes() ([]AdapterPrefix, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []AdapterPrefix
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if isDarwinVirtual(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4, ok := netip.AddrFromSlice(ipnet.IP.To4())
			if !ok {
				continue
			}
			ones, bits := ipnet.Mask.Size()
			if bits != 32 { // not a usable IPv4 mask
				continue
			}
			out = append(out, AdapterPrefix{
				Name:   iface.Name,
				Prefix: netip.PrefixFrom(ip4, ones).Masked(),
				IfType: darwinIfTypeEthernet,
				Up:     true,
			})
		}
	}
	return out, nil
}

// isDarwinVirtual reports whether name starts with one of the known macOS
// virtual-interface prefixes.
func isDarwinVirtual(name string) bool {
	for _, p := range darwinVirtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

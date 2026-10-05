// Package discover finds the local IPv4 subnets this machine sits on and
// enumerates the usable host addresses inside them.
package discover

import (
	"net/netip"

	"github.com/local/lancartographer/internal/winutil"
)

// adapterPrefixes is the seam between discovery policy and the Windows API;
// tests replace it with a fake.
var adapterPrefixes = winutil.AdapterPrefixes

// InterfaceInfo is a local network interface reduced to the subnet it sits
// on.
type InterfaceInfo struct {
	Name   string
	Prefix netip.Prefix
}

// Adapter interface types counted as physical. Anything else (tunnels, VPNs,
// WSL/Hyper-V virtual switches, loopback, ...) is treated as virtual.
const (
	ifTypeEthernet = 6  // IF_TYPE_ETHERNET_CSMACD
	ifTypeWiFi     = 71 // IF_TYPE_IEEE80211
)

func isPhysical(ifType uint32) bool {
	return ifType == ifTypeEthernet || ifType == ifTypeWiFi
}

// LocalSubnets returns the IPv4 subnets of adapters that are up, skipping
// loopback, link-local, and unspecified addresses. Virtual adapters are
// skipped unless includeVirtual is true.
func LocalSubnets(includeVirtual bool) ([]InterfaceInfo, error) {
	aps, err := adapterPrefixes()
	if err != nil {
		return nil, err
	}
	var out []InterfaceInfo
	for _, ap := range aps {
		if !ap.Up {
			continue
		}
		addr := ap.Prefix.Addr()
		if !addr.Is4() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
			continue
		}
		if !includeVirtual && !isPhysical(ap.IfType) {
			continue
		}
		out = append(out, InterfaceInfo{Name: ap.Name, Prefix: ap.Prefix})
	}
	return out, nil
}

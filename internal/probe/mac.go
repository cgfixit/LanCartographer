package probe

import (
	"fmt"
	"net/netip"

	"github.com/local/lancartographer/internal/winutil"
)

// MacNeedsElevation is the sentinel the engine writes into Host.MAC when a
// MAC lookup failed and the process is not elevated. The decision to use it
// belongs to the engine; probe only reports the error.
const MacNeedsElevation = "needs elevation"

// LookupMAC resolves the MAC address of the on-link IPv4 host at ip via
// SendARP and returns it colon-separated lowercase ("00:11:22:33:44:55").
// Any failure — unroutable target, non-IPv4 ip, unsupported platform —
// yields "" and a wrapped error; the caller decides the degrade.
func LookupMAC(ip netip.Addr) (string, error) {
	mac, err := winutil.SendARP(ip)
	if err != nil {
		return "", fmt.Errorf("probe: MAC lookup for %s: %w", ip, err)
	}
	return formatMAC(mac), nil
}

// formatMAC renders a 6-byte MAC as colon-separated lowercase hex.
func formatMAC(mac [6]byte) string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

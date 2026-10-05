//go:build !windows && !darwin

package winutil

import (
	"errors"
	"net/netip"
)

// SendARP is implemented only on Windows.
func SendARP(ip netip.Addr) (mac [6]byte, err error) {
	return mac, errors.New("winutil: SendARP is not supported on this platform")
}

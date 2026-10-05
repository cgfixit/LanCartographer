// Package winutil wraps the Windows APIs LanCartographer needs: network
// adapter enumeration and process elevation checks.
package winutil

import "net/netip"

// AdapterPrefix is one IPv4 unicast address on a network adapter, reduced to
// the adapter's friendly name, the on-link prefix, and the adapter's
// interface type and operational status.
type AdapterPrefix struct {
	Name   string
	Prefix netip.Prefix
	IfType uint32
	Up     bool
}

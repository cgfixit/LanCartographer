// Package maplayout computes a deterministic 2D layout of discovered hosts:
// the center host sits at the origin and every other host sits on a ring of
// radius 1.0, ordered by last IPv4 octet (ties broken by full IP).
package maplayout

import (
	"math"
	"net/netip"
	"slices"

	"github.com/local/lancartographer/internal/model"
)

// ringRadius is the distance of every non-center host from the origin.
const ringRadius = 1.0

// Layout maps each host's IP string to its 2D point. The host whose IP
// equals center is placed at {0, 0}; when no host matches, the center key is
// still emitted at the origin. All other hosts are placed on the ring at
// angle 2π·i/n, where i is the host's index after sorting by (last octet,
// full IP bytes). The result is a pure function of the host set and center:
// input order, rand, time, and I/O play no part.
func Layout(hosts []model.Host, center netip.Addr) map[string][2]float64 {
	out := make(map[string][2]float64, len(hosts)+1)
	out[center.String()] = [2]float64{0, 0}

	ring := make([]netip.Addr, 0, len(hosts))
	seen := make(map[netip.Addr]struct{}, len(hosts))
	for _, h := range hosts {
		if h.IP == center {
			continue
		}
		if _, dup := seen[h.IP]; dup {
			continue
		}
		seen[h.IP] = struct{}{}
		ring = append(ring, h.IP)
	}

	slices.SortFunc(ring, func(a, b netip.Addr) int {
		if la, lb := lastOctet(a), lastOctet(b); la != lb {
			return int(la) - int(lb)
		}
		return a.Compare(b)
	})

	n := float64(len(ring))
	for i, ip := range ring {
		angle := 2 * math.Pi * float64(i) / n
		out[ip.String()] = [2]float64{ringRadius * math.Cos(angle), ringRadius * math.Sin(angle)}
	}
	return out
}

// lastOctet returns the final address byte: the last IPv4 octet for v4 hosts,
// the last v6 byte otherwise (v6 hosts are not expected on the LAN scan, but
// the layout must stay total and deterministic for any input).
func lastOctet(a netip.Addr) byte {
	if a.Is4() || a.Is4In6() {
		b := a.As4()
		return b[3]
	}
	b := a.As16()
	return b[15]
}

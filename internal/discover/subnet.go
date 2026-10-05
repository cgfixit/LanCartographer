package discover

import "net/netip"

// HostsIn returns every usable host address in p, ascending. For prefixes
// longer than /31 the network and broadcast addresses are excluded; /31 and
// /32 prefixes return their contained addresses as-is.
func HostsIn(p netip.Prefix) []netip.Addr {
	if !p.IsValid() {
		return nil
	}
	p = p.Masked()
	first := p.Addr()
	last := lastAddr(p)
	if p.Bits() < 31 {
		first = first.Next()
		last = last.Prev()
	}
	var out []netip.Addr
	for a := first; a.IsValid() && a.Compare(last) <= 0; a = a.Next() {
		out = append(out, a)
	}
	return out
}

// lastAddr returns the highest address in p (the broadcast address for
// IPv4).
func lastAddr(p netip.Prefix) netip.Addr {
	p = p.Masked()
	bits := p.Bits()
	if p.Addr().Is4() {
		b := p.Addr().As4()
		for i := bits; i < 32; i++ {
			b[i/8] |= 1 << (7 - uint(i%8))
		}
		return netip.AddrFrom4(b)
	}
	b := p.Addr().As16()
	for i := bits; i < 128; i++ {
		b[i/8] |= 1 << (7 - uint(i%8))
	}
	return netip.AddrFrom16(b)
}

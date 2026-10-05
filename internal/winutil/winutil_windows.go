//go:build windows

package winutil

import (
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"
)

// AdapterPrefixes enumerates every IPv4 unicast address on the machine via
// GetAdaptersAddresses and converts each one to its masked on-link prefix.
func AdapterPrefixes() ([]AdapterPrefix, error) {
	size := uint32(15 * 1024)
	for {
		buf := make([]byte, size)
		head := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, head, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil, err
		}
		var out []AdapterPrefix
		for aa := head; aa != nil; aa = aa.Next {
			name := windows.UTF16PtrToString(aa.FriendlyName)
			up := aa.OperStatus == windows.IfOperStatusUp
			for ua := aa.FirstUnicastAddress; ua != nil; ua = ua.Next {
				sa := ua.Address.Sockaddr
				if sa == nil || sa.Addr.Family != windows.AF_INET {
					continue
				}
				raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(sa))
				prefix := netip.PrefixFrom(netip.AddrFrom4(raw.Addr), int(ua.OnLinkPrefixLength)).Masked()
				out = append(out, AdapterPrefix{
					Name:   name,
					Prefix: prefix,
					IfType: aa.IfType,
					Up:     up,
				})
			}
		}
		return out, nil
	}
}

// IsElevated reports whether the current process token is elevated. Any
// failure to inspect the token is treated as not elevated.
func IsElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	var elevated uint32
	var outLen uint32
	if err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), uint32(unsafe.Sizeof(elevated)), &outLen); err != nil {
		return false
	}
	return elevated != 0
}

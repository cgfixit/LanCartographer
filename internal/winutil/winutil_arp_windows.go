//go:build windows

package winutil

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modIphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	procSendARP = modIphlpapi.NewProc("SendARP")
)

// SendARP resolves the MAC address of the on-link IPv4 host at ip via the
// IP Helper SendARP call. destIP is the IPAddr ULONG in network byte order:
// on little-endian Windows that is the numeric value whose in-memory bytes
// are the address octets in order, i.e. LittleEndian.Uint32 of the Addr4
// array (the same value inet_addr returns). A non-IPv4 ip is rejected
// before the call. On failure the error carries the API return code (e.g.
// 67 ERROR_BAD_NET_NAME for an unreachable or off-link target).
func SendARP(ip netip.Addr) (mac [6]byte, err error) {
	if !ip.Is4() {
		return mac, fmt.Errorf("winutil: SendARP requires an IPv4 address, got %s", ip)
	}
	addr4 := ip.As4()
	destIP := binary.LittleEndian.Uint32(addr4[:])
	macLen := uint32(6)
	ret, _, _ := procSendARP.Call(
		uintptr(destIP),
		0, // srcIP: let the stack pick the source interface
		uintptr(unsafe.Pointer(&mac[0])),
		uintptr(unsafe.Pointer(&macLen)),
	)
	if ret != 0 {
		return mac, fmt.Errorf("winutil: SendARP(%s) failed with error %d", ip, ret)
	}
	if macLen < 6 {
		return mac, fmt.Errorf("winutil: SendARP(%s) returned short MAC length %d", ip, macLen)
	}
	return mac, nil
}

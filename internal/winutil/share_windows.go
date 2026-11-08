//go:build windows

package winutil

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modNetapi32          = windows.NewLazySystemDLL("netapi32.dll")
	procNetShareEnum     = modNetapi32.NewProc("NetShareEnum")
	procNetAPIBufferFree = modNetapi32.NewProc("NetApiBufferFree")
)

const (
	netShareEnumOK     = 0          // NERR_Success
	errAccessDenied    = 5          // ERROR_ACCESS_DENIED
	maxPreferredLength = 0xFFFFFFFF // let the API allocate one buffer holding all entries
)

// shareInfo1Raw mirrors the native SHARE_INFO_1 layout: two UTF-16 string
// pointers around a uint32 type field. Go's default alignment reproduces the
// C packing on both 32- and 64-bit Windows.
type shareInfo1Raw struct {
	netname *uint16
	typ     uint32
	remark  *uint16
}

// Keep a slot until the synchronous API returns, even after Stop. A new
// scan can skip SMB enrichment when earlier calls still occupy every slot.
var shareEnumSlots = make(chan struct{}, 64)

// NetShareEnum enumerates the shares advertised by the host at ip via
// netapi32 NetShareEnum. Only info level 1 is supported (name, type, remark).
// The current user token is used as-is; alternate credentials are never
// tried. ERROR_ACCESS_DENIED (a normal null-session result) maps to
// ErrAccessDenied.
func NetShareEnum(ctx context.Context, ip string, level int) ([]ShareInfo1, error) {
	ctx, cancel := context.WithTimeout(ctx, shareEnumTimeout)
	defer cancel()
	return awaitShareEnum(ctx, func() ([]ShareInfo1, error) { return netShareEnum(ip, level) })
}

func awaitShareEnum(ctx context.Context, enumerate func() ([]ShareInfo1, error)) ([]ShareInfo1, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case shareEnumSlots <- struct{}{}:
	default:
		return nil, fmt.Errorf("winutil: SMB enumeration is busy")
	}
	type result struct {
		shares []ShareInfo1
		err    error
	}
	done := make(chan result, 1)
	go func() {
		r := result{err: ctx.Err()}
		if r.err == nil {
			r.shares, r.err = enumerate()
		}
		<-shareEnumSlots
		done <- r
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.shares, r.err
	}
}

func netShareEnum(ip string, level int) ([]ShareInfo1, error) {
	if level != 1 {
		return nil, fmt.Errorf("winutil: NetShareEnum level %d not supported (level 1 only)", level)
	}

	server, err := windows.UTF16PtrFromString(`\\` + ip)
	if err != nil {
		return nil, fmt.Errorf("winutil: invalid server name: %w", err)
	}

	var buf *byte
	var entriesRead, totalEntries, resume uint32
	ret, _, _ := procNetShareEnum.Call(
		uintptr(unsafe.Pointer(server)), // serverName: \\ip, always passed
		uintptr(level),
		uintptr(unsafe.Pointer(&buf)), // buf: API-allocated out-buffer (must be NetApiBufferFree'd)
		maxPreferredLength,            // prefmaxlen: one buffer, no resume loop
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&totalEntries)),
		uintptr(unsafe.Pointer(&resume)), // unused with MAX_PREFERRED_LENGTH
	)

	// Free the API buffer exactly once, on every path past this point.
	// buf stays nil (free is skipped) when the call failed before allocation.
	if buf != nil {
		defer procNetAPIBufferFree.Call(uintptr(unsafe.Pointer(buf)))
	}

	switch ret {
	case netShareEnumOK:
	case errAccessDenied:
		return nil, ErrAccessDenied
	default:
		return nil, fmt.Errorf("winutil: NetShareEnum(%s) failed with error %d", ip, ret)
	}

	// Walk buf as entriesRead consecutive shareInfo1Raw records. The pointer
	// arithmetic below is the only unsafe stepping: base + i*stride, always
	// within the single API-allocated buffer, and buf outlives the reads
	// because the deferred free runs after this function returns.
	stride := unsafe.Sizeof(shareInfo1Raw{})
	out := make([]ShareInfo1, 0, entriesRead)
	for i := uint32(0); i < entriesRead; i++ {
		// Convert base + offset to a typed record pointer for this entry.
		rec := (*shareInfo1Raw)(unsafe.Pointer(uintptr(unsafe.Pointer(buf)) + uintptr(i)*stride))
		// UTF16PtrToString copies each field out of the API buffer into
		// Go-owned memory, so no Go pointer into buf survives the free.
		out = append(out, ShareInfo1{
			Name:   windows.UTF16PtrToString(rec.netname),
			Type:   rec.typ,
			Remark: windows.UTF16PtrToString(rec.remark),
		})
	}
	return out, nil
}

package probe

import (
	"context"
	"net/netip"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/winutil"
)

// ErrAccessDenied reports that the remote host rejected the current user
// token when enumerating shares (a null-session failure is a normal result,
// surfaced as "shares: access denied", not a crash). It aliases
// winutil.ErrAccessDenied so callers can errors.Is against either name.
var ErrAccessDenied = winutil.ErrAccessDenied

const (
	stypeSpecial  = 0x80000000 // hidden admin share (ADMIN$, C$, IPC$)
	stypeTypeMask = 0x0FFFFFFF // strips STYPE_SPECIAL and STYPE_TEMPORARY (0x40000000)
	stypeDiskTree = 0
	stypePrintQ   = 1
	stypeIPC      = 3
)

// ListShares enumerates the shares advertised by the host at ip via
// NetShareEnum info level 1 (name, type, remark — never share contents).
// Hidden admin shares (STYPE_SPECIAL) and non disk/print/ipc types are
// filtered out. An access-denied remote yields ErrAccessDenied.
func ListShares(ctx context.Context, ip netip.Addr) ([]model.Share, error) {
	infos, err := winutil.NetShareEnum(ctx, ip.String(), 1)
	if err != nil {
		return nil, err
	}
	return sharesFromInfos(ip.String(), infos), nil
}

// sharesFromInfos converts raw level-1 share records into model shares for
// the host at ip. It is pure (no API calls) so the mapping is fixture-tested.
func sharesFromInfos(ip string, infos []winutil.ShareInfo1) []model.Share {
	var out []model.Share
	for _, si := range infos {
		if si.Type&stypeSpecial != 0 {
			continue // hidden admin share
		}
		var typ string
		switch si.Type & stypeTypeMask {
		case stypeDiskTree:
			typ = model.ShareDisk
		case stypePrintQ:
			typ = model.SharePrint
		case stypeIPC:
			typ = model.ShareIPC
		default:
			continue // device or unknown type
		}
		out = append(out, model.Share{
			Name:   si.Name,
			Type:   typ,
			Remark: si.Remark,
			UNC:    `\\` + ip + `\` + si.Name,
		})
	}
	return out
}

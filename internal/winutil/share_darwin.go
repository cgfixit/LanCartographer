//go:build darwin

package winutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// NetShareEnum enumerates the shares advertised by the host at ip via
// `smbutil view //ip` (argv list, 5s timeout — never a shell string). Only
// info level 1 is supported, matching the Windows wrapper. The current user
// session is used as-is; alternate credentials are never tried. An smbutil
// authentication failure (a normal null-session result) maps to
// ErrAccessDenied; any other failure is a generic error carrying the tool's
// own message.
func NetShareEnum(ctx context.Context, ip string, level int) ([]ShareInfo1, error) {
	if level != 1 {
		return nil, fmt.Errorf("winutil: NetShareEnum level %d not supported (level 1 only)", level)
	}
	ctx, cancel := context.WithTimeout(ctx, shareEnumTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "smbutil", "view", "//"+ip).CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := strings.TrimSpace(string(out))
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "authentication") ||
			strings.Contains(lower, "logon") ||
			strings.Contains(lower, "access denied") {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("winutil: NetShareEnum(%s): smbutil view failed: %w: %s", ip, err, msg)
	}
	return parseSMBUtilView(string(out)), nil
}

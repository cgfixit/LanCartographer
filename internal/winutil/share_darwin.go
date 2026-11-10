//go:build darwin

package winutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// NetShareEnum enumerates the shares advertised by the host at ip via
// `smbutil view //ip` with a five-second timeout. The current user session
// is used as-is. An authentication failure maps to ErrAccessDenied.
// Other failures include the tool's own message.
func NetShareEnum(ctx context.Context, ip string) ([]ShareInfo1, error) {
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

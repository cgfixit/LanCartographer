//go:build darwin

package probe

import (
	"context"
	"net/netip"
	"os/exec"
	"time"
)

// ICMPAlive sends one echo request using macOS ping without elevating the app.
func ICMPAlive(ctx context.Context, ip netip.Addr) bool {
	if !ip.Is4() || ctx.Err() != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, "/sbin/ping", "-n", "-c", "1", "-W", "800", ip.String()).Run()
	return err == nil && ctx.Err() == nil
}

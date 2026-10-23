//go:build !darwin

package probe

import (
	"context"
	"net/netip"
)

// ICMPAlive is a macOS-only discovery fallback.
func ICMPAlive(context.Context, netip.Addr) bool { return false }

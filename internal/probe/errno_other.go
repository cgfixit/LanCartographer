//go:build !windows

package probe

import (
	"errors"
	"syscall"
)

// IsConnectionRefused reports an explicit refusal from the TCP dialer.
func IsConnectionRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }

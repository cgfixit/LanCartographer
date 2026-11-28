package probe

import (
	"errors"

	"golang.org/x/sys/windows"
)

// IsConnectionRefused reports an explicit refusal from the TCP dialer.
func IsConnectionRefused(err error) bool { return errors.Is(err, windows.WSAECONNREFUSED) }

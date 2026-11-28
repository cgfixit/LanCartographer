//go:build !windows

package servicecheck

import (
	"errors"
	"syscall"
)

func isUnreachable(err error) bool {
	return errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH)
}

func isReset(err error) bool { return errors.Is(err, syscall.ECONNRESET) }

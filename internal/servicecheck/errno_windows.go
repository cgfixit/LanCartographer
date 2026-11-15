//go:build windows

package servicecheck

import (
	"errors"
	"syscall"
)

// Winsock reports connection errors with its own codes, which do not match
// the syscall.E* values Go defines for Windows.
const (
	wsaeNetUnreach  = syscall.Errno(10051)
	wsaeConnReset   = syscall.Errno(10054)
	wsaeConnRefused = syscall.Errno(10061)
	wsaeHostUnreach = syscall.Errno(10065)
)

func isRefused(err error) bool { return errors.Is(err, wsaeConnRefused) }

func isUnreachable(err error) bool {
	return errors.Is(err, wsaeHostUnreach) || errors.Is(err, wsaeNetUnreach)
}

func isReset(err error) bool { return errors.Is(err, wsaeConnReset) }

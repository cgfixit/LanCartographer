//go:build !windows && !darwin

package winutil

import (
	"context"
	"errors"
)

// NetShareEnum is implemented on macOS and Windows.
func NetShareEnum(_ context.Context, ip string) ([]ShareInfo1, error) {
	return nil, errors.New("winutil: NetShareEnum is only supported on macOS and Windows")
}

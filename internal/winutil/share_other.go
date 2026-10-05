//go:build !windows

package winutil

import "errors"

// NetShareEnum is implemented only on Windows.
func NetShareEnum(ip string, level int) ([]ShareInfo1, error) {
	return nil, errors.New("winutil: NetShareEnum is only supported on Windows")
}

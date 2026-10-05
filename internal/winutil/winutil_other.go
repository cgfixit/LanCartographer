//go:build !windows && !darwin

package winutil

import "errors"

// AdapterPrefixes is implemented only on Windows.
func AdapterPrefixes() ([]AdapterPrefix, error) {
	return nil, errors.New("winutil: AdapterPrefixes is only supported on Windows")
}

// IsElevated is implemented only on Windows; elsewhere it reports false.
func IsElevated() bool {
	return false
}

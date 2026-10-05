//go:build !windows && !darwin

package winutil

import "errors"

// RelaunchElevated is implemented only on Windows; elsewhere it reports
// that elevation is unsupported.
func RelaunchElevated() error {
	return errors.New("winutil: elevation relaunch is only supported on Windows")
}

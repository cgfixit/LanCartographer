//go:build windows

package winutil

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// RelaunchElevated starts a copy of the current executable with the
// "runas" verb so Windows prompts the operator for elevation. It returns
// as soon as ShellExecute reports back (an error here means the prompt was
// declined or failed); the caller is expected to exit so only the elevated
// instance keeps running.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("winutil: cannot locate own executable: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("winutil: runas verb: %w", err)
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("winutil: executable path: %w", err)
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_NORMAL); err != nil {
		return fmt.Errorf("winutil: elevation request declined or failed: %w", err)
	}
	return nil
}

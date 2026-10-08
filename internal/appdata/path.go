// Package appdata resolves the per-user directory shared by the GUI and CLI.
package appdata

import (
	"fmt"
	"os"
	"path/filepath"
)

// Path returns a file below LanCartographer's configuration directory.
// An explicit APPDATA remains supported for Windows and isolated fixtures;
// otherwise UserConfigDir selects the native directory (Application Support
// on macOS). Missing or relative configuration must never fall back to cwd.
func Path(name string) (string, error) {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		var err error
		dir, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("configuration directory must be absolute: %q", dir)
	}
	return filepath.Join(dir, "LanCartographer", name), nil
}

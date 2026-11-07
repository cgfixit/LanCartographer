//go:build !windows

package export

import "os"

func restrictReplacement(_ *os.File, _ string) error {
	// CreateTemp uses mode 0600 for raw inventory.
	return nil
}

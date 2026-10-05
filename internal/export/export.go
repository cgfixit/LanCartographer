// Package export writes host inventories to CSV or JSON files on disk.
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/local/lancartographer/internal/model"
)

// ToFile writes hosts to path in the given format ("csv" or "json",
// case-insensitive). Parent directories are created as needed.
func ToFile(path string, hosts []model.Host, format string) error {
	var write func(w *os.File, hosts []model.Host) error
	switch strings.ToLower(format) {
	case "csv":
		write = func(w *os.File, h []model.Host) error { return model.WriteCSV(w, h) }
	case "json":
		write = func(w *os.File, h []model.Host) error { return model.WriteJSON(w, h) }
	default:
		return fmt.Errorf("export: unknown format %q (want csv or json)", format)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(f, hosts); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

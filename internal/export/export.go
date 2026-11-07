// Package export writes host inventories to CSV or JSON files on disk.
package export

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/local/lancartographer/internal/model"
)

// ToFile writes hosts to path in the given format ("csv" or "json",
// case-insensitive). Parent directories are created as needed.
func ToFile(path string, hosts []model.Host, format string) error {
	var write func(io.Writer, []model.Host) error
	switch strings.ToLower(format) {
	case "csv":
		write = model.WriteCSV
	case "json":
		write = model.WriteJSON
	default:
		return fmt.Errorf("export: unknown format %q (want csv or json)", format)
	}
	return writeFile(path, func(w io.Writer) error { return write(w, hosts) })
}

func writeFile(path string, write func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	existing, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return err
	case !existing.Mode().IsRegular():
		return fmt.Errorf("export: destination %q is not a regular file", path)
	default:
		// A directory can allow replacement even when the file is unwritable.
		if existing.Mode().Perm()&0o222 == 0 {
			return &os.PathError{Op: "replace", Path: path, Err: os.ErrPermission}
		}
		dst, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		if err := dst.Close(); err != nil {
			return err
		}
	}

	f, err := os.CreateTemp(dir, ".inventory-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if existing != nil {
		if err := restrictReplacement(f, path); err != nil {
			f.Close()
			return err
		}
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

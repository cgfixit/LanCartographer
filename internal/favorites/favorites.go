// Package favorites persists the user's favorite hosts as JSON under
// %AppData%\LanCartographer\favorites.json.
package favorites

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/local/lancartographer/internal/model"
)

// path returns the favorites file location for the current APPDATA.
func path() string {
	return filepath.Join(os.Getenv("APPDATA"), "LanCartographer", "favorites.json")
}

// Load reads the saved favorites. A missing file is not an error and
// yields (nil, nil); a corrupt file is an error.
func Load() ([]model.Host, error) {
	f, err := os.Open(path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var hosts []model.Host
	if err := json.NewDecoder(f).Decode(&hosts); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return hosts, nil
}

// Save writes hosts as indented JSON, creating the directory if needed.
// The write goes through a temp file renamed into place so a crash
// mid-write cannot leave a truncated favorites.json.
func Save(hosts []model.Host) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "favorites-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := model.WriteJSON(tmp, hosts); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

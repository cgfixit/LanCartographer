// Package favorites persists the user's favorite hosts as JSON under
// the per-user LanCartographer configuration directory.
package favorites

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/local/lancartographer/internal/appdata"
	"github.com/local/lancartographer/internal/model"
)

// path returns the favorites file location for the current user.
func path() (string, error) {
	return appdata.Path("favorites.json")
}

// Load returns IPv4 favorites and the number skipped. A missing file is empty.
func Load() ([]model.Host, int, error) {
	p, err := path()
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var hosts []model.Host
	if err := json.NewDecoder(f).Decode(&hosts); err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	total := len(hosts)
	hosts = slices.DeleteFunc(hosts, func(h model.Host) bool { return !h.IP.Is4() })
	return hosts, total - len(hosts), nil
}

// Save writes hosts as indented JSON, creating the directory if needed.
// The write goes through a temp file renamed into place so a crash
// mid-write cannot leave a truncated favorites.json.
func Save(hosts []model.Host) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "favorites-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := model.WriteJSON(tmp, hosts); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}

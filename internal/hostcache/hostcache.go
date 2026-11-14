// Package hostcache persists MAC-keyed hostnames as JSON under
// the per-user LanCartographer directory, so a device that stays silent to
// every name probe (IoT, phones) keeps the name it resolved under last time.
package hostcache

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/local/lancartographer/internal/appdata"
)

// Entry is one cached hostname: the name and when it was last observed.
type Entry struct {
	Name string `json:"name"`
	Last int64  `json:"last"` // unix seconds
}

// Map is the MAC-keyed hostname cache. Keys are lower-case colon MACs.
type Map map[string]Entry

// ttl is how long a cached name is trusted without re-confirmation.
const ttl = int64((30 * 24 * time.Hour) / time.Second)

// path returns the names file location for the current user.
func path() (string, error) {
	return appdata.Path("names.json")
}

// Load reads the saved cache. Like favorites it fails soft: a missing or
// corrupt file yields an empty map and a nil error — a forgotten name is
// re-resolved on the next scan, never surfaced as a failure.
func Load() (Map, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Map{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var m Map
	if err := json.NewDecoder(f).Decode(&m); err != nil && !errors.Is(err, io.EOF) {
		return Map{}, nil
	}
	if m == nil {
		m = Map{}
	}
	return m, nil
}

// Save writes m as indented JSON, creating the directory if needed. The
// write goes through a temp file renamed into place so a crash mid-write
// cannot leave a truncated names.json.
func Save(m Map) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "names-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := json.NewEncoder(tmp).Encode(m); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}

// Get returns the cached name for mac when an entry exists and was seen
// within the last 30 days. mac is case-insensitive.
func (m Map) Get(mac string) (string, bool) {
	e, ok := m[strings.ToLower(mac)]
	if !ok || time.Now().Unix()-e.Last > ttl {
		return "", false
	}
	return e.Name, true
}

// Put records name for mac as of now. mac is stored lower-case.
func (m Map) Put(mac, name string) {
	m[strings.ToLower(mac)] = Entry{Name: name, Last: time.Now().Unix()}
}

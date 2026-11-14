package gui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/local/lancartographer/internal/appdata"
)

// AppSettings is the persisted user preference set, stored as JSON under
// the per-user LanCartographer directory. Theme is "dark" or "light".
type AppSettings struct {
	Theme string `json:"theme"`
}

// settingsPath returns the settings file location for the current user.
func settingsPath() (string, error) {
	return appdata.Path("settings.json")
}

// LoadSettings reads the saved settings. It fails soft: a missing file, a
// corrupt file, or an unrecognized theme value all yield the defaults
// (light theme) with a nil error, so a bad settings file never blocks
// startup.
func LoadSettings() (AppSettings, error) {
	def := AppSettings{Theme: "light"}
	p, err := settingsPath()
	if err != nil {
		return def, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		// Missing AND unreadable settings both fall back to defaults by
		// design — a bad file must never block startup.
		return def, nil //nolint:nilerr
	}
	var s AppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return def, nil
	}
	if s.Theme != "dark" && s.Theme != "light" {
		return def, nil
	}
	return s, nil
}

// SaveSettings writes the settings as JSON, creating the directory if
// needed. The write goes through a temp file renamed into place so a crash
// mid-write cannot leave a truncated settings.json.
func SaveSettings(s AppSettings) error {
	p, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "settings-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := json.NewEncoder(tmp).Encode(s); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}

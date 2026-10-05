package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppSettings is the persisted user preference set, stored as JSON under
// %AppData%\LanCartographer\settings.json. Theme is "dark" or "light".
type AppSettings struct {
	Theme string `json:"theme"`
}

// settingsPath returns the settings file location for the current APPDATA.
func settingsPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "LanCartographer", "settings.json")
}

// LoadSettings reads the saved settings. It fails soft: a missing file, a
// corrupt file, or an unrecognized theme value all yield the defaults
// (light theme) with a nil error, so a bad settings file never blocks
// startup.
func LoadSettings() (AppSettings, error) {
	def := AppSettings{Theme: "light"}
	data, err := os.ReadFile(settingsPath())
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
	p := settingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "settings-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := json.NewEncoder(tmp).Encode(s); err != nil {
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

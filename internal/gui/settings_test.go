package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettingsMissingFileDefaultsLight(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings on missing file: %v", err)
	}
	if s.Theme != "light" {
		t.Fatalf("Theme = %q, want light", s.Theme)
	}
}

func TestLoadSettingsCorruptFileDefaultsLight(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	p := filepath.Join(dir, "LanCartographer", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings on corrupt file: %v", err)
	}
	if s.Theme != "light" {
		t.Fatalf("Theme = %q, want light", s.Theme)
	}
}

func TestSaveThenLoadSettingsRoundTrip(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := SaveSettings(AppSettings{Theme: "dark"}); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Theme != "dark" {
		t.Fatalf("Theme = %q, want dark", s.Theme)
	}
}

func TestLoadSettingsUnknownThemeDefaultsLight(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	p := filepath.Join(dir, "LanCartographer", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"theme":"plaid"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Theme != "light" {
		t.Fatalf("Theme = %q, want light", s.Theme)
	}
}

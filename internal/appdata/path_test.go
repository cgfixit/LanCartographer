package appdata

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathNativeDirectory(t *testing.T) {
	t.Setenv("APPDATA", "")
	if runtime.GOOS == "windows" {
		// UserConfigDir itself requires APPDATA on Windows. Missing it
		// must fail rather than writing into the working directory.
		if got, err := Path("settings.json"); err == nil || got != "" {
			t.Fatalf("Path without APPDATA = %q, %v", got, err)
		}
		return
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Path("settings.json")
	if err != nil || got != filepath.Join(dir, "LanCartographer", "settings.json") {
		t.Fatalf("Path = %q, %v", got, err)
	}
}

func TestPathExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	got, err := Path("favorites.json")
	if err != nil || got != filepath.Join(dir, "LanCartographer", "favorites.json") {
		t.Fatalf("Path = %q, %v", got, err)
	}
}

func TestPathRejectsRelativeDirectory(t *testing.T) {
	t.Setenv("APPDATA", "relative")
	if got, err := Path("names.json"); err == nil || got != "" {
		t.Fatalf("Path = %q, %v; must not write relative to cwd", got, err)
	}
}

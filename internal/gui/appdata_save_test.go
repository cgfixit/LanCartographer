package gui

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/local/lancartographer/internal/favorites"
	"github.com/local/lancartographer/internal/hostcache"
	"github.com/local/lancartographer/internal/model"
)

func TestAppDataSaveCleanup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		save   func(bool) error
		before string
		after  string
	}{
		{"favorites.json", func(replace bool) error {
			ip := netip.MustParseAddr("192.0.2.1")
			if replace {
				ip = netip.MustParseAddr("192.0.2.2")
			}
			return favorites.Save([]model.Host{{IP: ip}})
		}, "192.0.2.1", "192.0.2.2"},
		{"names.json", func(replace bool) error {
			name := "original"
			if replace {
				name = "replacement"
			}
			return hostcache.Save(hostcache.Map{"02:00:00:00:00:01": {Name: name, Last: 1}})
		}, "original", "replacement"},
		{"settings.json", func(replace bool) error {
			theme := "light"
			if replace {
				theme = "dark"
			}
			return SaveSettings(AppSettings{Theme: theme})
		}, "light", "dark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("APPDATA", root)
			dir := filepath.Join(root, "LanCartographer")
			dest := filepath.Join(dir, tc.name)
			if err := os.MkdirAll(dest, 0o700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(dest, "keep")
			if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			assertOnlyDestination := func() {
				t.Helper()
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.Name() != tc.name {
						t.Errorf("unexpected file after save: %s", entry.Name())
					}
				}
			}
			for range 2 {
				if err := tc.save(false); err == nil {
					t.Fatal("saving over a nonempty directory succeeded")
				}
				if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
					t.Fatalf("failed save changed the destination: %q, %v", data, err)
				}
				assertOnlyDestination()
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(dest); err != nil {
				t.Fatal(err)
			}
			for i, want := range []string{tc.before, tc.after} {
				if err := tc.save(i == 1); err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(dest); err != nil || !bytes.Contains(data, []byte(want)) {
					t.Fatalf("saved file = %q, %v; want %q", data, err, want)
				}
				assertOnlyDestination()
			}
			info, err := os.Stat(dest)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Errorf("file mode = %o, want 600", info.Mode().Perm())
			}
		})
	}
}

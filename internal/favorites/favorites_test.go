package favorites

import (
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/local/lancartographer/internal/model"
)

func testHosts() []model.Host {
	return []model.Host{
		{
			IP:       netip.MustParseAddr("192.168.1.10"),
			Alive:    true,
			FoundVia: "ping",
			Name:     "nas",
			MAC:      "00:11:22:33:44:55",
			Vendor:   "Acme",
			Ports: []model.Port{
				{Number: 80, State: model.PortOpen, Service: "http", Title: "NAS UI"},
				{Number: 445, State: model.PortOpen, Service: "smb"},
			},
			Shares: []model.Share{
				{Name: "DATA", Type: model.ShareDisk, Remark: "media", UNC: `\\192.168.1.10\DATA`},
			},
		},
		{
			IP:       netip.MustParseAddr("192.168.1.20"),
			Alive:    false,
			FoundVia: "arp",
			Name:     "printer",
		},
	}
}

func TestLoadMissingFileReturnsNilNil(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	hosts, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing file: got error %v, want nil", err)
	}
	if hosts != nil {
		t.Fatalf("Load() on missing file: got %v, want nil", hosts)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	want := testHosts()

	if err := Save(want); err != nil {
		t.Fatalf("Save(): %v", err)
	}

	path := filepath.Join(dir, "LanCartographer", "favorites.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected favorites.json at %s: %v", path, err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestSaveOverwritesPrevious(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	if err := Save(testHosts()); err != nil {
		t.Fatalf("Save() first: %v", err)
	}
	second := testHosts()[:1]
	if err := Save(second); err != nil {
		t.Fatalf("Save() second: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !reflect.DeepEqual(got, second) {
		t.Fatalf("after overwrite:\n got: %+v\nwant: %+v", got, second)
	}
}

func TestLoadCorruptJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	sub := filepath.Join(dir, "LanCartographer")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "favorites.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("Load() on corrupt JSON: got nil error, want error")
	}
}

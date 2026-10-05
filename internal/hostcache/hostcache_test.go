package hostcache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func useTempAppData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	return dir
}

func TestLoad_MissingFile(t *testing.T) {
	useTempAppData(t)
	m, err := Load()
	if err != nil {
		t.Fatalf("Load = %v, want nil error for missing file", err)
	}
	if len(m) != 0 {
		t.Errorf("Load = %v, want empty map", m)
	}
}

func TestLoad_CorruptFile(t *testing.T) {
	dir := useTempAppData(t)
	p := filepath.Join(dir, "LanCartographer", "names.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load()
	if err != nil {
		t.Fatalf("Load = %v, want nil error for corrupt file (fail soft)", err)
	}
	if len(m) != 0 {
		t.Errorf("Load = %v, want empty map", m)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	useTempAppData(t)
	m := Map{"aa:bb:cc:dd:ee:ff": {Name: "office-printer", Last: 12345}}
	if err := Save(m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got["aa:bb:cc:dd:ee:ff"].Name != "office-printer" || got["aa:bb:cc:dd:ee:ff"].Last != 12345 {
		t.Errorf("round trip = %v, want the saved entry back", got)
	}
}

func TestSave_CreatesDir(t *testing.T) {
	dir := useTempAppData(t)
	if err := Save(Map{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "LanCartographer", "names.json")); err != nil {
		t.Errorf("names.json not created: %v", err)
	}
}

func TestGet_TTL(t *testing.T) {
	day := int64(24 * time.Hour / time.Second)
	m := Map{
		"aa:bb:cc:dd:ee:01": {Name: "recent", Last: time.Now().Unix() - 29*day},
		"aa:bb:cc:dd:ee:02": {Name: "stale", Last: time.Now().Unix() - 31*day},
	}
	if name, ok := m.Get("aa:bb:cc:dd:ee:01"); !ok || name != "recent" {
		t.Errorf("Get(recent) = %q, %v; want recent, true", name, ok)
	}
	if name, ok := m.Get("aa:bb:cc:dd:ee:02"); ok || name != "" {
		t.Errorf("Get(stale) = %q, %v; want \"\", false after 30 days", name, ok)
	}
	if _, ok := m.Get("aa:bb:cc:dd:ee:99"); ok {
		t.Error("Get(unknown) = true, want false")
	}
}

func TestGet_NormalizesMACCase(t *testing.T) {
	m := Map{"aa:bb:cc:dd:ee:ff": {Name: "box", Last: time.Now().Unix()}}
	if name, ok := m.Get("AA:BB:CC:DD:EE:FF"); !ok || name != "box" {
		t.Errorf("Get(upper) = %q, %v; want box, true", name, ok)
	}
}

func TestPut_RecordsNow(t *testing.T) {
	m := Map{}
	m.Put("AA:BB:CC:DD:EE:FF", "box")
	e, ok := m["aa:bb:cc:dd:ee:ff"]
	if !ok {
		t.Fatalf("Put stored key %v, want lowercase", m)
	}
	if e.Name != "box" {
		t.Errorf("Name = %q, want box", e.Name)
	}
	if e.Last == 0 || time.Since(time.Unix(e.Last, 0)) > time.Minute {
		t.Errorf("Last = %d, want a fresh unix timestamp", e.Last)
	}
}

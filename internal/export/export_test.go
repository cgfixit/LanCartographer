package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
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
				{Number: 80, State: model.PortOpen, Service: "http"},
				{Number: 445, State: model.PortOpen, Service: "smb"},
			},
			Shares: []model.Share{
				{Name: "DATA", Type: model.ShareDisk, UNC: `\\192.168.1.10\DATA`},
			},
		},
		{
			IP:    netip.MustParseAddr("192.168.1.20"),
			Alive: false,
			Name:  "printer",
		},
	}
}

func TestToFileJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "out.json")
	if err := ToFile(path, testHosts(), "json"); err != nil {
		t.Fatalf("ToFile(json): %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got []model.Host
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("output is not a JSON host array: %v", err)
	}
	if len(got) != 2 || got[0].Name != "nas" || got[1].IP.String() != "192.168.1.20" {
		t.Fatalf("unexpected JSON content: %+v", got)
	}
}

func TestToFileCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	if err := ToFile(path, testHosts(), "csv"); err != nil {
		t.Fatalf("ToFile(csv): %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("output is not CSV: %v", err)
	}
	if len(rows) != 3 { // header + 2 hosts
		t.Fatalf("got %d CSV rows, want 3", len(rows))
	}
	if rows[0][0] != "ip" || rows[1][0] != "192.168.1.10" || rows[2][0] != "192.168.1.20" {
		t.Fatalf("unexpected CSV content: %v", rows)
	}
}

func TestToFileFormatCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	for _, format := range []string{"JSON", "Csv"} {
		path := filepath.Join(dir, "out-"+format)
		if err := ToFile(path, testHosts(), format); err != nil {
			t.Fatalf("ToFile(%q): %v", format, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("ToFile(%q) did not create file: %v", format, err)
		}
	}
}

func TestToFileUnknownFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.xml")
	err := ToFile(path, testHosts(), "xml")
	if err == nil {
		t.Fatal("ToFile(xml): got nil error, want error")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Fatalf("error should name the bad format, got: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("unknown format must not create a file, stat: %v", statErr)
	}
}

func TestToFileCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "out.json")
	if err := ToFile(path, testHosts(), "json"); err != nil {
		t.Fatalf("ToFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created under nested parents: %v", err)
	}
}

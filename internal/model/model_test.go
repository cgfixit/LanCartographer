package model

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func testHost() Host {
	return Host{
		IP:       netip.MustParseAddr("192.168.1.50"),
		Alive:    true,
		FoundVia: "arp",
		Name:     "fileserv",
		MAC:      "aa:bb:cc:dd:ee:ff",
		Vendor:   "Dell Inc.",
		Ports: []Port{
			{Number: 80, State: "open", Service: "http", Title: "nginx"},
			{Number: 445, State: "open", Service: "smb"},
			{Number: 22, State: "closed", Service: "ssh"},
		},
		Shares: []Share{
			{Name: "DATA", Type: "disk", Remark: "main", UNC: `\\192.168.1.50\DATA`},
			{Name: "print$", Type: "print", UNC: `\\192.168.1.50\print$`},
		},
	}
}

func TestOpenPortsSummary(t *testing.T) {
	h := testHost()
	got := h.OpenPortsSummary()
	want := "80 open http, 445 open smb"
	if got != want {
		t.Errorf("OpenPortsSummary() = %q, want %q", got, want)
	}

	empty := Host{Ports: []Port{{Number: 22, State: "closed", Service: "ssh"}}}
	if got := empty.OpenPortsSummary(); got != "" {
		t.Errorf("OpenPortsSummary() with no open ports = %q, want %q", got, "")
	}
}

func TestSharesSummary(t *testing.T) {
	h := testHost()
	got := h.SharesSummary()
	want := `2 shares (first \\192.168.1.50\DATA)`
	if got != want {
		t.Errorf("SharesSummary() = %q, want %q", got, want)
	}

	empty := Host{}
	if got := empty.SharesSummary(); got != "0 shares" {
		t.Errorf("SharesSummary() with no shares = %q, want %q", got, "0 shares")
	}
}

func TestHasWebAndHasRDP(t *testing.T) {
	h := testHost()
	if !h.HasWeb() {
		t.Error("HasWeb() = false, want true (port 80 open)")
	}
	if h.HasRDP() {
		t.Error("HasRDP() = true, want false")
	}
	rdp := Host{Ports: []Port{{Number: 3389, State: "open", Service: "ms-wbt-server"}}}
	if !rdp.HasRDP() {
		t.Error("HasRDP() = false, want true (port 3389 open)")
	}
	closed := Host{Ports: []Port{{Number: 443, State: "closed"}}}
	if closed.HasWeb() {
		t.Error("HasWeb() = true, want false (443 closed)")
	}
}

func TestWriteJSONRoundTrip(t *testing.T) {
	hosts := []Host{testHost()}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, hosts); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var back []Host
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back, hosts) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", back, hosts)
	}
}

func TestWriteCSV(t *testing.T) {
	hosts := []Host{testHost()}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, hosts); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	out := buf.String()

	r := csv.NewReader(strings.NewReader(out))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (header + 1 host)", len(rows))
	}
	header := strings.Join(rows[0], ",")
	wantHeader := "ip,alive,name,mac,vendor,found_via,ports,shares"
	if header != wantHeader {
		t.Errorf("header = %q, want %q", header, wantHeader)
	}
	row := rows[1]
	if row[6] != "80 open http, 445 open smb" {
		t.Errorf("ports field = %q", row[6])
	}
	if row[7] != `2 shares (first \\192.168.1.50\DATA)` {
		t.Errorf("shares field = %q", row[7])
	}
	if !strings.Contains(out, `\\192.168.1.50\DATA`) {
		t.Errorf("CSV output does not contain UNC: %q", out)
	}
	// Ports field contains commas and must be quoted in raw output.
	if !strings.Contains(out, `"80 open http, 445 open smb"`) {
		t.Errorf("ports field not quoted in raw output: %q", out)
	}
}

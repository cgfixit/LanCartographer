package probe

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/local/lancartographer/internal/winutil"
)

// shareTypeSpecial mirrors STYPE_SPECIAL (0x80000000): set on hidden admin
// shares like ADMIN$ and C$.
const shareTypeSpecial = 0x80000000

func TestSharesFromInfos(t *testing.T) {
	infos := []winutil.ShareInfo1{
		{Name: "DATA", Type: 0, Remark: "Media files"},                       // disk
		{Name: "HPLaser", Type: 1, Remark: ""},                               // print queue
		{Name: "IPC", Type: 3, Remark: "Remote IPC"},                         // ipc without special bit
		{Name: "ADMIN$", Type: 0 | shareTypeSpecial, Remark: "Remote Admin"}, // hidden admin share
		{Name: "C$", Type: 0 | shareTypeSpecial, Remark: "Default share"},    // hidden admin share
		{Name: "TEMP", Type: 0 | 0x40000000, Remark: "temp"},                 // STYPE_TEMPORARY masked off → disk
		{Name: "DEV", Type: 2, Remark: "device"},                             // STYPE_DEVICE → not disk/print/ipc
		{Name: "WEIRD", Type: 9, Remark: ""},                                 // unknown type → excluded
	}

	got := sharesFromInfos("192.168.1.50", infos)

	want := []struct {
		name, typ, remark, unc string
	}{
		{"DATA", "disk", "Media files", `\\192.168.1.50\DATA`},
		{"HPLaser", "print", "", `\\192.168.1.50\HPLaser`},
		{"IPC", "ipc", "Remote IPC", `\\192.168.1.50\IPC`},
		{"TEMP", "disk", "temp", `\\192.168.1.50\TEMP`},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d shares, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Type != w.typ || got[i].Remark != w.remark || got[i].UNC != w.unc {
			t.Errorf("share %d = %+v, want {%q %q %q %q}", i, got[i], w.name, w.typ, w.remark, w.unc)
		}
	}
}

// TestSharesFromInfosEmpty ensures a host with no visible shares yields nil
// (not an empty-but-non-nil slice), keeping the "0 shares" summary clean.
func TestSharesFromInfosEmpty(t *testing.T) {
	if got := sharesFromInfos("10.0.0.1", nil); got != nil {
		t.Errorf("expected nil for no visible shares, got %+v", got)
	}
}

// TestShareJSONShape guards the v1 contract: a marshaled share carries only
// Name, Type, Remark, UNC — never any file-listing field.
func TestShareJSONShape(t *testing.T) {
	got := sharesFromInfos("192.168.1.50", []winutil.ShareInfo1{{Name: "DATA", Type: 0, Remark: "r"}})
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 share, got %d", len(decoded))
	}
	wantKeys := map[string]bool{"Name": true, "Type": true, "Remark": true, "UNC": true}
	for k := range decoded[0] {
		if !wantKeys[k] {
			t.Errorf("share JSON contains unexpected field %q (file-listing fields are out of scope)", k)
		}
	}
}

// TestErrAccessDeniedSentinel pins the engine contract: probe.ErrAccessDenied
// must match winutil.ErrAccessDenied via errors.Is.
func TestErrAccessDeniedSentinel(t *testing.T) {
	if !errors.Is(ErrAccessDenied, winutil.ErrAccessDenied) {
		t.Error("probe.ErrAccessDenied does not wrap/match winutil.ErrAccessDenied")
	}
}

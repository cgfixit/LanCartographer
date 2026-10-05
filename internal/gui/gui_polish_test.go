package gui

import (
	"net/netip"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
)

func TestEmptyHintIdleThenHiddenWithHosts(t *testing.T) {
	u := newTestUI(t)

	if !u.emptyHint.Visible() {
		t.Fatal("empty hint should be visible on a fresh window")
	}
	if u.emptyTitle.Text != "No hosts yet" {
		t.Fatalf("empty title = %q, want %q", u.emptyTitle.Text, "No hosts yet")
	}
	if !strings.Contains(u.emptySub.Text, "192.168.1.0/24") {
		t.Fatalf("empty sub = %q, want it to name the combo subnet", u.emptySub.Text)
	}

	setTwoHosts(u)
	if u.emptyHint.Visible() {
		t.Fatal("empty hint should hide once hosts exist")
	}
}

func TestEmptyHintScanningState(t *testing.T) {
	u := newTestUI(t)
	u.scanning = true
	u.updateEmptyHint()
	if !u.emptyHint.Visible() {
		t.Fatal("empty hint should be visible while scanning an empty store")
	}
	if !strings.HasPrefix(u.emptyTitle.Text, "Scanning ") {
		t.Fatalf("empty title = %q, want a Scanning prefix", u.emptyTitle.Text)
	}
	u.scanning = false
	u.updateEmptyHint()
	if u.emptyTitle.Text != "No hosts yet" {
		t.Fatalf("empty title after scan end = %q, want %q", u.emptyTitle.Text, "No hosts yet")
	}
}

// findText walks the object tree for a canvas.Text with the given content.
func findText(obj fyne.CanvasObject, want string) bool {
	if t, ok := obj.(*canvas.Text); ok && t.Text == want {
		return true
	}
	for _, c := range children(obj) {
		if findText(c, want) {
			return true
		}
	}
	return false
}

func TestPanelShowsSectionHeaders(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)
	u.selectedIP = "192.168.1.10"
	u.showSelectedHost()

	for _, section := range []string{"Identity", "Ports", "Shares"} {
		found := false
		for _, obj := range u.panel.Objects {
			if findText(obj, section) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("panel missing %q section header", section)
		}
	}
}

func TestPanelShareRowKeepsOpenAction(t *testing.T) {
	u := newTestUI(t)
	u.store.set([]model.Host{{
		IP:     netip.MustParseAddr("192.168.1.10"),
		Alive:  true,
		Name:   "alpha",
		Shares: []model.Share{{Name: "media", Type: "disk", Remark: "movies", UNC: `\\192.168.1.10\media`}},
	}})
	u.selectedIP = "192.168.1.10"
	u.refreshTable()
	u.showSelectedHost()

	if !findLabel(u.panel, "media") {
		t.Fatal("panel missing bold share name")
	}
	if !findText(u.panel, `\\192.168.1.10\media`) {
		t.Fatal("panel missing UNC caption")
	}
	if !findButton(u.panel, "Open") {
		t.Fatal("panel missing per-share Open action")
	}
}

func findLabel(obj fyne.CanvasObject, want string) bool {
	if l, ok := obj.(*widget.Label); ok && l.Text == want {
		return true
	}
	for _, c := range children(obj) {
		if findLabel(c, want) {
			return true
		}
	}
	return false
}

func findButton(obj fyne.CanvasObject, want string) bool {
	if b, ok := obj.(*widget.Button); ok && b.Text == want {
		return true
	}
	for _, c := range children(obj) {
		if findButton(c, want) {
			return true
		}
	}
	return false
}

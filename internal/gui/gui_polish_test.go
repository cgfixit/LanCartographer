//go:build cgo

package gui

import (
	"net/netip"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
)

func TestTapBoxPrimaryTapFiresHandler(t *testing.T) {
	box := newTapBox(widget.NewLabel("x"))
	fired := false
	box.onPrimary = func() { fired = true }
	test.Tap(box)
	if !fired {
		t.Fatal("onPrimary did not fire on Tapped")
	}

	// A nil onPrimary (panel children) must not panic.
	test.Tap(newTapBox(widget.NewLabel("y")))
}

func TestTablePrimaryTapHitsCellAndSelectsRow(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)

	pos := nameCellTapPos(t, u, 0)
	hit := tapCellAt(t, u, pos)

	box, ok := hit.(*tapBox)
	if !ok {
		t.Fatalf("primary tap dispatched to %T, want the *tapBox cell", hit)
	}
	if box.onPrimary == nil {
		t.Fatal("table cell tapBox has no onPrimary wired")
	}
	if u.selectedIP != "192.168.1.10" {
		t.Fatalf("selectedIP = %q, want 192.168.1.10", u.selectedIP)
	}
	title, ok := u.panel.Objects[0].(*widget.Label)
	if !ok || title.Text != "alpha" {
		t.Fatalf("panel did not switch to the tapped host (first object %v)", u.panel.Objects[0])
	}
}

func TestEmptyHintIdleThenHiddenWithHosts(t *testing.T) {
	u := newTestUI(t)

	if !u.emptyHint.Visible() {
		t.Fatal("empty hint should be visible on a fresh window")
	}
	if u.table.Visible() {
		t.Fatal("table should be hidden while the store is empty")
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
	if !u.table.Visible() {
		t.Fatal("table should be visible once hosts exist")
	}
}

func TestEmptyHintScanningState(t *testing.T) {
	u := newTestUI(t)
	u.scanning = true
	u.updateEmptyHint()
	if !u.emptyHint.Visible() {
		t.Fatal("empty hint should be visible while scanning an empty store")
	}
	if u.table.Visible() {
		t.Fatal("table should be hidden while scanning an empty store")
	}
	if !strings.HasPrefix(u.emptyTitle.Text, "Scanning ") {
		t.Fatalf("empty title = %q, want a Scanning prefix", u.emptyTitle.Text)
	}
	u.scanning = false
	u.updateEmptyHint()
	if u.emptyTitle.Text != "No hosts yet" {
		t.Fatalf("empty title after scan end = %q, want %q", u.emptyTitle.Text, "No hosts yet")
	}
	if u.table.Visible() {
		t.Fatal("table should stay hidden after a scan that found no hosts")
	}
}

func TestEmptyHintLayersOverTableBody(t *testing.T) {
	u := newTestUI(t)

	// The hint must be a sibling layer over u.table inside one stack — that
	// stack is what tableAreaLayout sizes into the body region.
	body, ok := u.split.Leading.(*fyne.Container)
	if !ok {
		t.Fatalf("split leading is %T, want *fyne.Container", u.split.Leading)
	}
	var stack *fyne.Container
	for _, obj := range body.Objects {
		c, ok := obj.(*fyne.Container)
		if !ok || len(c.Objects) < 2 {
			continue
		}
		if c.Objects[0] == u.table && c.Objects[1] == u.emptyHint {
			stack = c
		}
	}
	if stack == nil {
		t.Fatal("no table-below-hint stack layer found in the table area")
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

func TestNoMapPlaceholder(t *testing.T) {
	u := newTestUI(t)
	if findButton(u.win.Canvas().Content(), "Map — v1.1") {
		t.Fatal("map placeholder button is still in the window")
	}
	if findText(u.win.Canvas().Content(), "The 2D network map ships in v1.1.") {
		t.Fatal("map placeholder copy is still in the window")
	}
}

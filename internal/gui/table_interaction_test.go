//go:build cgo

package gui

import (
	"net/netip"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/discover"
	"github.com/local/lancartographer/internal/favorites"
	"github.com/local/lancartographer/internal/model"
)

// newTestUI builds the full window UI against fake deps: one local subnet,
// never elevated. No scan ever runs; tests inject hosts via store.set.
func newTestUI(t *testing.T) *ui {
	t.Helper()
	// Keep LoadSettings/SaveSettings off the real %APPDATA% settings.json.
	t.Setenv("APPDATA", t.TempDir())
	a := test.NewApp()
	w := a.NewWindow("test")
	w.Resize(fyne.NewSize(1120, 680))
	u := newUI(a, w, guiDeps{
		localSubnets: func(bool) ([]discover.InterfaceInfo, error) {
			return []discover.InterfaceInfo{
				{Name: "eth0", Prefix: netip.MustParsePrefix("192.168.1.0/24")},
			}, nil
		},
		isElevated: func() bool { return true },
	})
	return u
}

// children returns the direct visual children of obj: a container's objects
// or a widget's renderer objects (both positioned relative to obj).
func children(obj fyne.CanvasObject) []fyne.CanvasObject {
	switch o := obj.(type) {
	case *fyne.Container:
		return o.Objects
	case fyne.Widget:
		return test.WidgetRenderer(o).Objects()
	}
	return nil
}

// absPos locates target in the canvas object tree and returns its absolute
// position within the window canvas.
func absPos(tree fyne.CanvasObject, target fyne.CanvasObject) (fyne.Position, bool) {
	var rec func(obj fyne.CanvasObject, base fyne.Position) (fyne.Position, bool)
	rec = func(obj fyne.CanvasObject, base fyne.Position) (fyne.Position, bool) {
		p := base.Add(obj.Position())
		if obj == target {
			return p, true
		}
		for _, c := range children(obj) {
			if cp, ok := rec(c, p); ok {
				return cp, true
			}
		}
		return fyne.Position{}, false
	}
	return rec(tree, fyne.NewPos(0, 0))
}

// dispatch walks the visible tree deepest-first, exactly like the real
// driver, and delivers a tap to the deepest object at abs matching match.
// Returns the object tapped, or nil when nothing matched.
func dispatch(tree fyne.CanvasObject, abs fyne.Position, match func(fyne.CanvasObject) bool) fyne.CanvasObject {
	var rec func(obj fyne.CanvasObject, base fyne.Position) fyne.CanvasObject
	rec = func(obj fyne.CanvasObject, base fyne.Position) fyne.CanvasObject {
		if !obj.Visible() {
			return nil
		}
		p := base.Add(obj.Position())
		kids := children(obj)
		for i := len(kids) - 1; i >= 0; i-- {
			if hit := rec(kids[i], p); hit != nil {
				return hit
			}
		}
		in := abs.X >= p.X && abs.X < p.X+obj.Size().Width &&
			abs.Y >= p.Y && abs.Y < p.Y+obj.Size().Height
		if in && match(obj) {
			return obj
		}
		return nil
	}
	return rec(tree, fyne.NewPos(0, 0))
}

// tapCellAt taps the table cell area at table-local pos through the full
// dispatch walk (same path a real mouse click takes).
func tapCellAt(t *testing.T, u *ui, pos fyne.Position) fyne.CanvasObject {
	t.Helper()
	ap, ok := absPos(u.win.Canvas().Content(), u.table)
	if !ok {
		t.Fatal("table not found in canvas tree")
	}
	abs := ap.Add(pos)
	o := dispatch(u.win.Canvas().Content(), abs, func(o fyne.CanvasObject) bool {
		_, tap := o.(fyne.Tappable)
		return tap
	})
	if o == nil {
		t.Fatalf("no Tappable found at %v", abs)
	}
	o.(fyne.Tappable).Tapped(&fyne.PointEvent{Position: abs.Subtract(ap), AbsolutePosition: abs})
	return o
}

// secondaryTapCellAt is the right-button equivalent of tapCellAt.
func secondaryTapCellAt(t *testing.T, u *ui, pos fyne.Position) fyne.CanvasObject {
	t.Helper()
	ap, ok := absPos(u.win.Canvas().Content(), u.table)
	if !ok {
		t.Fatal("table not found in canvas tree")
	}
	abs := ap.Add(pos)
	o := dispatch(u.win.Canvas().Content(), abs, func(o fyne.CanvasObject) bool {
		_, ok := o.(fyne.SecondaryTappable)
		return ok
	})
	if o == nil {
		t.Fatalf("no SecondaryTappable found at %v", abs)
	}
	o.(fyne.SecondaryTappable).TappedSecondary(&fyne.PointEvent{Position: abs.Subtract(ap), AbsolutePosition: abs})
	return o
}

// nameCellTapPos returns the table-local center of row's Name cell (col 1).
// The X offset comes from the col-1 header button's laid-out position
// relative to the table (both are sized by tableAreaLayout from the same
// computeColWidths pass); row height is the table's template cell height.
func nameCellTapPos(t *testing.T, u *ui, row int) fyne.Position {
	t.Helper()
	tp, ok := absPos(u.win.Canvas().Content(), u.table)
	if !ok {
		t.Fatal("table not found in canvas tree")
	}
	hp, ok := absPos(u.win.Canvas().Content(), u.headerBtns[1])
	if !ok {
		t.Fatal("header button not found in canvas tree")
	}
	x := hp.X - tp.X + u.headerBtns[1].Size().Width/2
	h := u.table.CreateCell().MinSize().Height
	y := float32(row)*h + h/2
	return fyne.NewPos(x, y)
}

func setTwoHosts(u *ui) {
	u.store.set([]model.Host{
		{IP: netip.MustParseAddr("192.168.1.10"), Alive: true, Name: "alpha"},
		{IP: netip.MustParseAddr("192.168.1.20"), Alive: true, Name: "bravo"},
	})
	u.refreshTable()
}

func TestTableTapSelectsRowAndUpdatesPanel(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)

	// Sort is col 2 (IP) ascending, so row 0 is .10 / "alpha".
	pos := nameCellTapPos(t, u, 0)
	hit := tapCellAt(t, u, pos)

	if u.selectedIP != "192.168.1.10" {
		t.Fatalf("selectedIP = %q, want 192.168.1.10 (hit %T at %v)", u.selectedIP, hit, pos)
	}
	if len(u.panel.Objects) == 0 {
		t.Fatal("panel is empty after selection")
	}
	if _, isPlaceholder := u.panel.Objects[0].(*fyne.Container); isPlaceholder {
		t.Fatal("panel still shows placeholder after row tap")
	}
	title, ok := u.panel.Objects[0].(*widget.Label)
	if !ok {
		t.Fatalf("panel first object is %T, want title *widget.Label", u.panel.Objects[0])
	}
	if title.Text == "Select a host to see details." {
		t.Fatal("panel still shows placeholder hint after row tap")
	}
	if title.Text != "alpha" {
		t.Fatalf("panel title = %q, want alpha", title.Text)
	}
}

func TestTableTapSelectsNewHostInSameCellAfterSort(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)
	pos := nameCellTapPos(t, u, 0)
	tapCellAt(t, u, pos)

	u.headerBtns[2].OnTapped() // reverse IP order so bravo replaces alpha in row 0
	if u.selectedIP != "192.168.1.10" {
		t.Fatalf("sort changed the selected host to %q", u.selectedIP)
	}

	tapCellAt(t, u, pos)
	if title := u.panel.Objects[0].(*widget.Label).Text; title != "bravo" {
		t.Fatalf("panel title after tapping the same cell = %q, want bravo", title)
	}
}

func TestTableNameSortKeepsFavoritesInIPOrder(t *testing.T) {
	u := newTestUI(t)
	low := netip.MustParseAddr("192.168.1.2")
	middle := netip.MustParseAddr("192.168.1.10")
	high := netip.MustParseAddr("192.168.1.20")
	u.store.set([]model.Host{
		{IP: middle, Name: "bravo"},
		{IP: high, Name: "Alpha"},
		{IP: low, Name: "alpha"},
	})
	order := func(hosts []model.Host) []netip.Addr {
		ips := make([]netip.Addr, 0, len(hosts))
		for _, h := range hosts {
			ips = append(ips, h.IP)
		}
		return ips
	}

	u.headerBtns[1].OnTapped()
	if got := order(u.sorted); !slices.Equal(got, []netip.Addr{low, high, middle}) {
		t.Fatalf("ascending Name order = %v", got)
	}
	u.headerBtns[1].OnTapped()
	if got := order(u.sorted); !slices.Equal(got, []netip.Addr{middle, high, low}) {
		t.Fatalf("descending Name order = %v", got)
	}
	u.onSaveFavorites()
	saved, _, err := favorites.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := order(saved); !slices.Equal(got, []netip.Addr{low, middle, high}) {
		t.Fatalf("saved favorites are out of IP order: %v", got)
	}
}

func TestTableSecondaryTapOpensMenu(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)

	pos := nameCellTapPos(t, u, 0)
	secondaryTapCellAt(t, u, pos)

	if n := len(u.win.Canvas().Overlays().List()); n == 0 {
		t.Fatal("no overlay shown after secondary tap on row")
	}
}

func TestLoadFavoritesAllowsReselectingSameCell(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)
	u.onSaveFavorites()
	id := widget.TableCellID{Row: 0, Col: 2}
	u.table.Select(id)

	u.onLoadFavorites()
	if u.selectedIP != "" {
		t.Fatal("loading favorites retained the previous host selection")
	}
	u.table.Select(id)
	if u.selectedIP != "192.168.1.10" {
		t.Fatal("selecting the same cell after reloading did not reopen the host")
	}
}

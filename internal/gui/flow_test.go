//go:build cgo

package gui

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
)

func TestToolbarHoldsTheMainWorkflow(t *testing.T) {
	u := newTestUI(t)
	for _, want := range []string{"Scan", "Stop", "Check Service", "Sessions", "Export", "Dark theme"} {
		if !findButton(u.topBar, want) {
			t.Errorf("toolbar lacks %q", want)
		}
	}
	for _, gone := range []string{"Save", "Load", "Light on"} {
		if findButton(u.topBar, gone) {
			t.Errorf("toolbar still has %q", gone)
		}
	}
	var file []string
	for _, m := range u.mainMenu().Items {
		if m.Label == "File" {
			for _, item := range m.Items {
				file = append(file, item.Label)
			}
		}
	}
	if !strings.Contains(strings.Join(file, "|"), "Save Favorites|Load Favorites") {
		t.Fatalf("File menu lost the favorites actions: %v", file)
	}
}

func TestIdleProgressHiddenAndStatusReadable(t *testing.T) {
	u := newTestUI(t)
	if u.progress.Visible() {
		t.Fatal("progress bar should be hidden before any scan")
	}
	if u.statusElaps.Text != "Ready" || u.statusHosts.Text != "0 hosts" {
		t.Fatalf("idle status = %q / %q", u.statusElaps.Text, u.statusHosts.Text)
	}
	u.store.set([]model.Host{{IP: netip.MustParseAddr("192.168.1.10"), Alive: true,
		Ports: []model.Port{{Number: 22, State: model.PortOpen}}}})
	u.refreshTable()
	if u.statusHosts.Text != "1 host" || u.statusPorts.Text != "1 open port" {
		t.Fatalf("singular status = %q / %q", u.statusHosts.Text, u.statusPorts.Text)
	}

	u.started = time.Now()
	u.progress.Show()
	u.scanDone = true
	u.finishScan()
	if u.progress.Visible() || !strings.HasSuffix(u.statusElaps.Text, "s") || strings.Contains(u.statusElaps.Text, "·") {
		t.Fatalf("after completion: progress visible=%v status=%q", u.progress.Visible(), u.statusElaps.Text)
	}
	u.scanDone = false
	u.finishScan()
	if !strings.HasPrefix(u.statusElaps.Text, "Stopped · ") {
		t.Fatalf("after stop: status=%q", u.statusElaps.Text)
	}
}

func TestEmptyHintAfterScanWithNoHosts(t *testing.T) {
	u := newTestUI(t)
	u.lastPrefix = netip.MustParsePrefix("192.168.1.0/24")
	u.updateEmptyHint()
	if u.emptyTitle.Text != "No hosts answered in 192.168.1.0/24" || !strings.Contains(u.emptySub.Text, "scan again") {
		t.Fatalf("empty hint = %q / %q", u.emptyTitle.Text, u.emptySub.Text)
	}
}

// cellParts builds and updates one table cell and returns its row
// background and label.
func cellParts(u *ui, row, col int) (*canvas.Rectangle, *widget.Label) {
	obj := u.table.CreateCell()
	u.table.UpdateCell(widget.TableCellID{Row: row, Col: col}, obj)
	inner := obj.(*tapBox).content.(*fyne.Container)
	return inner.Objects[0].(*canvas.Rectangle), inner.Objects[1].(*widget.Label)
}

func TestSelectedRowHighlightsEveryColumnAndDimsUnknowns(t *testing.T) {
	u := newTestUI(t)
	setTwoHosts(u)
	u.table.Select(widget.TableCellID{Row: 0, Col: 1})
	selection := theme.Color(theme.ColorNameSelection)
	for col := 1; col < len(tableColumns); col++ {
		if bg, _ := cellParts(u, 0, col); bg.FillColor != selection {
			t.Fatalf("selected row col %d background = %v, want selection color", col, bg.FillColor)
		}
		if bg, _ := cellParts(u, 1, col); bg.FillColor == selection {
			t.Fatalf("unselected row col %d is highlighted", col)
		}
	}
	_, mac := cellParts(u, 0, 3)
	if mac.Text != emptyCell || mac.Importance != widget.LowImportance {
		t.Fatalf("unknown MAC renders %q importance %v", mac.Text, mac.Importance)
	}
	_, name := cellParts(u, 0, 1)
	if name.Text != "alpha" || name.Importance != widget.MediumImportance {
		t.Fatalf("known name renders %q importance %v", name.Text, name.Importance)
	}
}

func TestPanelListsOpenPortsFirstAndSummarizesTheRest(t *testing.T) {
	u := newTestUI(t)
	u.store.set([]model.Host{{IP: netip.MustParseAddr("192.168.1.10"), Alive: true, FoundVia: "tcp",
		Ports: []model.Port{
			{Number: 80, State: model.PortClosed},
			{Number: 443, State: model.PortOpen, Service: "Web", Title: "Router"},
			{Number: 445, State: model.PortTimeout},
			{Number: 22, State: model.PortOpen, Service: "SSH"},
		}}})
	u.selectedIP = "192.168.1.10"
	u.showSelectedHost()
	for _, want := range []string{"Ports · 2 open of 4 probed", "22  SSH", "443  Web  —  Router", "Closed: 80", "No answer: 445", "TCP connection", emptyCell} {
		if !findLabel(u.panel, want) {
			t.Errorf("panel lacks %q", want)
		}
	}
	checks := 0
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "Check…" {
			checks++
		}
		for _, c := range children(o) {
			walk(c)
		}
	}
	walk(u.panel)
	if checks != 2 || !findButton(u.panel, "Check service…") {
		t.Fatalf("Check… buttons = %d (want one per open port) and host-level action present=%v", checks, findButton(u.panel, "Check service…"))
	}
}

func TestPortCheckOnlyPrefills(t *testing.T) {
	u := newTestUI(t)
	h := model.Host{IP: netip.MustParseAddr("192.168.1.10"), Alive: true,
		Ports: []model.Port{{Number: 3000, State: model.PortOpen, Service: "Web"}}}
	row := u.portRow(h, h.Ports[0])
	var check *widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "Check…" {
			check = b
		}
		for _, c := range children(o) {
			walk(c)
		}
	}
	walk(row)
	if check == nil {
		t.Fatal("open port row has no Check… button")
	}
	check.OnTapped()
	v := u.serviceView
	if v == nil {
		t.Fatal("Check… did not open Check Service")
	}
	t.Cleanup(v.win.Close)
	if v.host.Text != "192.168.1.10" || v.port.Text != "3000" || v.protocol.Selected != "HTTP" || v.job != nil {
		t.Fatalf("prefill = %q %q %q job=%v", v.host.Text, v.port.Text, v.protocol.Selected, v.job)
	}
}

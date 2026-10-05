package gui

import (
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
)

// tableColumns names the seven table columns; index 0 is the alive/dead
// status circle and has no header text.
var tableColumns = []string{"", "Name", "IP", "MAC", "Vendor", "Ports", "Shares"}

// colWidths sizes the seven table columns; the header row uses the same
// widths so headers line up with their columns.
var colWidths = []float32{44, 170, 130, 160, 160, 260, 220}

// aliveGreen / deadRed paint the status circle.
var (
	aliveGreen  = color.NRGBA{R: 0x2e, G: 0xcc, B: 0x71, A: 0xff}
	deadRed     = color.NRGBA{R: 0xe7, G: 0x4c, B: 0x3c, A: 0xff}
	transparent = color.NRGBA{A: 0}
)

// buildTable creates the sortable inventory table plus its clickable
// header row. Column 0 is a colored alive/dead circle.
func (u *ui) buildTable() {
	u.table = widget.NewTable(
		func() (int, int) { return len(u.sorted), len(tableColumns) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Truncation = fyne.TextTruncateEllipsis
			circle := canvas.NewCircle(transparent)
			box := newTapBox(container.NewStack(label, container.NewCenter(
				container.NewGridWrap(fyne.NewSize(12, 12), circle))))
			box.onSecondary = func(e *fyne.PointEvent) { u.showRowMenu(box.id, e) }
			return box
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			box := obj.(*tapBox)
			box.id = id
			box.hostIP = ""
			inner := box.CanvasObject.(*fyne.Container)
			label := inner.Objects[0].(*widget.Label)
			circleC := inner.Objects[1].(*fyne.Container)
			circle := circleC.Objects[0].(*fyne.Container).Objects[0].(*canvas.Circle)
			if id.Row >= len(u.sorted) {
				return
			}
			h := u.sorted[id.Row]
			box.hostIP = h.IP.String()
			if id.Col == 0 {
				label.Hide()
				circleC.Show()
				if h.Alive {
					circle.FillColor = aliveGreen
				} else {
					circle.FillColor = deadRed
				}
				circle.Refresh()
				return
			}
			circleC.Hide()
			label.Show()
			label.SetText(cellText(h, id.Col))
		},
	)
	for i, w := range colWidths {
		u.table.SetColumnWidth(i, w)
	}
	u.table.OnSelected = func(id widget.TableCellID) {
		if id.Row >= len(u.sorted) {
			return
		}
		u.selectedIP = u.sorted[id.Row].IP.String()
		u.showSelectedHost()
	}

	u.headerBtns = make([]*widget.Button, len(tableColumns))
	header := make([]fyne.CanvasObject, len(tableColumns))
	for col := range tableColumns {
		if col == 0 {
			header[col] = container.NewGridWrap(fyne.NewSize(colWidths[col], 36), canvas.NewRectangle(transparent))
			continue
		}
		col := col
		btn := widget.NewButton(u.headerText(col), func() { u.onHeaderClick(col) })
		u.headerBtns[col] = btn
		header[col] = container.NewGridWrap(fyne.NewSize(colWidths[col], 36), btn)
	}
	u.split = container.NewHSplit(
		container.NewBorder(container.NewHBox(header...), nil, nil, nil, u.table),
		container.NewVScroll(u.panel))
	u.split.SetOffset(0.7)
}

// onHeaderClick toggles or sets the sort column and re-sorts.
func (u *ui) onHeaderClick(col int) {
	if u.sortCol == col {
		u.sortAsc = !u.sortAsc
	} else {
		u.sortCol = col
		u.sortAsc = true
	}
	u.refreshTable()
	u.refreshHeader()
}

// refreshHeader updates the sort arrows on the header buttons.
func (u *ui) refreshHeader() {
	for col, btn := range u.headerBtns {
		if btn == nil {
			continue
		}
		btn.SetText(u.headerText(col))
	}
}

// headerText renders a column header with its sort arrow.
func (u *ui) headerText(col int) string {
	name := tableColumns[col]
	if u.sortCol != col {
		return name
	}
	if u.sortAsc {
		return name + " ▲"
	}
	return name + " ▼"
}

// cellText renders one non-status table cell.
func cellText(h model.Host, col int) string {
	switch col {
	case 1:
		return h.Name
	case 2:
		return h.IP.String()
	case 3:
		return h.MAC
	case 4:
		return h.Vendor
	case 5:
		return h.OpenPortsSummary()
	case 6:
		return sharesCell(h)
	}
	return ""
}

// sortHosts returns a copy of hosts ordered per the sort column: status
// (alive first), name, IP, MAC, vendor, open-port count, then share count.
// Ties always fall back to IP so the order stays deterministic.
func sortHosts(hosts []model.Host, col int, asc bool) []model.Host {
	out := append([]model.Host(nil), hosts...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		c := 0
		switch col {
		case 0:
			if a.Alive != b.Alive {
				c = boolCmp(b.Alive, a.Alive) // alive first
			}
		case 1:
			c = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case 2:
			c = a.IP.Compare(b.IP)
		case 3:
			c = strings.Compare(a.MAC, b.MAC)
		case 4:
			c = strings.Compare(strings.ToLower(a.Vendor), strings.ToLower(b.Vendor))
		case 5:
			c = openCount(a) - openCount(b)
		case 6:
			c = len(a.Shares) - len(b.Shares)
		}
		if c == 0 {
			c = a.IP.Compare(b.IP)
		}
		if !asc {
			return c > 0
		}
		return c < 0
	})
	return out
}

// boolCmp orders false before true.
func boolCmp(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// openCount counts open ports on a host.
func openCount(h model.Host) int {
	n := 0
	for _, p := range h.Ports {
		if p.State == model.PortOpen {
			n++
		}
	}
	return n
}

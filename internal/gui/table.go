package gui

import (
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
)

// tableColumns names the seven table columns; index 0 is the alive/dead
// status circle and has no header text.
var tableColumns = []string{"", "Name", "IP", "MAC", "Vendor", "Ports", "Shares"}

// colFractions gives each of the seven columns its share of the available
// width; the fractions must sum to exactly 1.0. colMins floors each column
// so it never collapses below a readable width (the table h-scrolls when
// the minimums exceed the available width).
var colFractions = []float32{0.045, 0.155, 0.115, 0.145, 0.135, 0.225, 0.18}
var colMins = []float32{40, 110, 95, 130, 100, 150, 120}

// computeColWidths splits avail across the columns by fraction, clamping
// any column that would fall below its minimum and redistributing the
// leftover among the unclamped columns (water-filling), so the sum never
// exceeds avail unless every column sits at its minimum.
func computeColWidths(avail float32) []float32 {
	widths := make([]float32, len(colFractions))
	fixed := make([]bool, len(colFractions))
	remaining, remFrac := avail, float32(1)
	for {
		clamped := false
		for i := range colFractions {
			if fixed[i] {
				continue
			}
			if colFractions[i]/remFrac*remaining < colMins[i] {
				fixed[i] = true
				widths[i] = colMins[i]
				remaining -= colMins[i]
				remFrac -= colFractions[i]
				clamped = true
			}
		}
		if !clamped {
			break
		}
	}
	for i := range colFractions {
		if !fixed[i] {
			widths[i] = colFractions[i] / remFrac * remaining
		}
	}
	return widths
}

// headerRowHeight is the header button strip height; tableTopOffset is the
// header height plus the small gap before the table body.
const (
	headerRowHeight = 36
	tableTopOffset  = headerRowHeight + 2
)

// tableAreaLayout lays out the header row above the table body and
// re-derives the table's column widths from the area's actual width on
// every layout pass, so the columns track window resizes instead of
// staying fixed. body is the table plus its empty-state overlay stack; it
// is moved and sized exactly where the bare table used to sit.
type tableAreaLayout struct {
	header []fyne.CanvasObject
	table  *widget.Table
	body   fyne.CanvasObject
	minSum float32
}

// MinSize reports the floor: the column minimums side by side, tall enough
// for the header plus a usable table body.
func (l *tableAreaLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.minSum, headerRowHeight+200)
}

// Layout positions the header cells to match the freshly computed column
// widths and sizes the body (table + overlay) to fill the rest of the area.
func (l *tableAreaLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	widths := computeColWidths(size.Width)
	x := float32(0)
	for i := range widths {
		if i < len(l.header) {
			l.header[i].Move(fyne.NewPos(x, 0))
			l.header[i].Resize(fyne.NewSize(widths[i], headerRowHeight))
		}
		x += widths[i]
		l.table.SetColumnWidth(i, widths[i])
	}
	l.body.Move(fyne.NewPos(0, tableTopOffset))
	l.body.Resize(fyne.NewSize(size.Width, fyne.Max(size.Height-tableTopOffset, 0)))
}

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
			inner := box.content.(*fyne.Container)
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
			header[col] = canvas.NewRectangle(transparent)
			continue
		}
		col := col
		btn := widget.NewButton(u.headerText(col), func() { u.onHeaderClick(col) })
		btn.Alignment = widget.ButtonAlignLeading
		u.headerBtns[col] = btn
		header[col] = btn
	}
	minSum := float32(0)
	for _, m := range colMins {
		minSum += m
	}

	// Empty-state guidance layered over the TABLE BODY only (stacked
	// table-below-hint), so it centers within the body region rather than
	// below the header row. A solid card behind the text keeps the table's
	// column separators from showing through; applyTheme recolors it.
	u.emptyTitle = widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	u.emptySub = widget.NewLabel("")
	u.emptySub.Alignment = fyne.TextAlignCenter
	u.emptyCard = canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	u.emptyHint = container.NewCenter(container.NewStack(u.emptyCard,
		container.NewPadded(container.NewVBox(u.emptyTitle, u.emptySub))))
	body := container.NewStack(u.table, u.emptyHint)

	area := container.New(
		&tableAreaLayout{header: header, table: u.table, body: body, minSum: minSum},
		append(append([]fyne.CanvasObject{}, header...), body)...)

	u.split = container.NewHSplit(area, container.NewVScroll(u.panel))
	u.split.SetOffset(0.7)
}

// updateEmptyHint swaps the empty-table overlay between the idle guidance
// and the scanning notice, hiding it as soon as any host exists. Called
// from refreshTable and finishScan.
func (u *ui) updateEmptyHint() {
	if u.store.len() > 0 {
		u.emptyHint.Hide()
		return
	}
	if u.scanning {
		u.emptyTitle.SetText("Scanning " + strings.TrimSpace(u.subnet.Text) + "…")
		u.emptySub.SetText("")
	} else {
		target := strings.TrimSpace(u.subnet.Text)
		if target == "" {
			target = "your subnet"
		}
		u.emptyTitle.SetText("No hosts yet")
		u.emptySub.SetText("Click Scan to inventory " + target)
	}
	u.emptyHint.Show()
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

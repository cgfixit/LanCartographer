//go:build cgo

package gui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/macdiag"
	"github.com/local/lancartographer/internal/session"
)

type connectionsUI struct {
	view *connectionsView
	job  *connectionJob
}

type connectionJob struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (c *connectionsUI) stopAndWait() {
	if job := c.job; job != nil {
		job.cancel()
		<-job.done
	}
}

var connectionHeaders = [...]string{"Process", "PID", "Family", "Protocol", "Local endpoint", "Peer endpoint", "TCP state"}

type connectionsView struct {
	owner              *ui
	win                fyne.Window
	filter             *widget.Entry
	kind               *widget.Select
	refresh, stop      *widget.Button
	jump, copy         *widget.Button
	status, count      *widget.Label
	details, inventory *widget.Label
	progress           *widget.ProgressBarInfinite
	table              *widget.Table
	rows, visible      []macdiag.Connection
	selected           *macdiag.Connection
	sample             time.Time
	sortColumn         int
	reverse            bool
}

func (u *ui) connectionsMenu() []*fyne.MenuItem {
	return []*fyne.MenuItem{fyne.NewMenuItem("Connections…", func() { u.showConnections(netip.Addr{}) })}
}

func (u *ui) connectionHostMenu(ip netip.Addr) []*fyne.MenuItem {
	if !ip.Is4() {
		return nil
	}
	return []*fyne.MenuItem{fyne.NewMenuItem("Filter connections to this address…", func() { u.showConnections(ip) })}
}

func (u *ui) showConnections(ip netip.Addr) {
	if v := u.connections.view; v != nil {
		if ip.IsValid() {
			v.kind.SetSelected("All")
			v.filter.SetText(ip.String())
		}
		v.win.RequestFocus()
		return
	}
	v := &connectionsView{
		owner: u, win: u.app.NewWindow("Connections — LanCartographer"),
		filter: widget.NewEntry(), status: widget.NewLabel("Ready · Select Refresh to sample."),
		count: widget.NewLabel(""), details: widget.NewLabel("Select a socket to inspect its endpoints."),
		inventory: widget.NewLabel(""), progress: widget.NewProgressBarInfinite(),
	}
	u.connections.view = v
	v.details.Wrapping = fyne.TextWrapWord
	v.filter.SetPlaceHolder("Filter by process, PID, address or port")
	v.filter.OnChanged = func(string) { v.refreshRows() }
	v.kind = widget.NewSelect([]string{"All", "Connected", "Listening", "UDP"}, func(string) { v.refreshRows() })
	v.kind.Selected = "All"
	v.refresh = widget.NewButton("Refresh", v.takeSnapshot)
	v.refresh.Importance = widget.HighImportance
	v.stop = widget.NewButton("Stop", v.cancel)
	v.stop.Disable()
	v.progress.Hide()
	v.jump = widget.NewButton("Show in inventory", v.showInInventory)
	v.copy = widget.NewButton("Copy selected row", func() {
		if v.selected != nil {
			u.app.Clipboard().SetContent(strings.Join(connectionCells(*v.selected), "\t"))
		}
	})
	v.buildTable()
	if ip.IsValid() {
		v.filter.SetText(ip.String())
	}
	v.refreshSelection()
	if u.connections.job != nil {
		v.refresh.Disable()
		v.status.SetText("Waiting for the previous helper to stop…")
	}
	intro := widget.NewLabel("Visible connections on this Mac. Access depends on your account and system permissions.")
	intro.Wrapping = fyne.TextWrapWord
	top := container.NewVBox(intro, container.NewBorder(nil, nil, v.kind, nil, v.filter), v.count)
	bottom := container.NewVBox(v.details, v.inventory, v.status, v.progress,
		container.NewHBox(v.refresh, v.stop, v.jump, v.copy, widget.NewButton("Close", v.win.Close)))
	v.win.SetContent(container.NewPadded(container.NewBorder(top, bottom, nil, nil, v.table)))
	v.win.Resize(fyne.NewSize(940, 540))
	v.win.SetOnClosed(func() {
		v.progress.Stop()
		u.connections.view = nil
		v.cancel()
	})
	v.win.Show()
}

func connectionCells(row macdiag.Connection) []string {
	peer := "—"
	if row.HasPeer {
		peer = row.Peer.String()
	}
	state := row.State
	if state == "" {
		state = "—"
	}
	name := row.Process
	if name == "" {
		name = "Unavailable"
	}
	return []string{name, strconv.Itoa(row.PID), row.Family, row.Protocol, row.Local.String(), peer, state}
}

func (v *connectionsView) buildTable() {
	label := func() fyne.CanvasObject {
		l := widget.NewLabel("")
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	v.table = widget.NewTable(func() (int, int) { return len(v.visible), len(connectionHeaders) }, label,
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row < len(v.visible) {
				obj.(*widget.Label).SetText(connectionCells(v.visible[id.Row])[id.Col])
			}
		})
	v.table.ShowHeaderRow = true
	v.table.CreateHeader = func() fyne.CanvasObject {
		btn := widget.NewButton("", nil)
		btn.Alignment = widget.ButtonAlignLeading
		return btn
	}
	v.table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		btn := obj.(*widget.Button)
		text := connectionHeaders[id.Col]
		if id.Col == v.sortColumn {
			text += " ▲"
			if v.reverse {
				text = connectionHeaders[id.Col] + " ▼"
			}
		}
		btn.SetText(text)
		btn.OnTapped = func() {
			v.reverse = v.sortColumn == id.Col && !v.reverse
			v.sortColumn = id.Col
			v.refreshRows()
		}
	}
	for col, width := range [...]float32{132, 62, 68, 77, 195, 195, 153} {
		v.table.SetColumnWidth(col, width)
	}
	v.table.OnSelected = func(id widget.TableCellID) {
		if id.Row < len(v.visible) {
			row := v.visible[id.Row]
			v.selected = &row
		}
		v.refreshSelection()
	}
}

func compareConnection(a, b macdiag.Connection, column int) int {
	switch column {
	case 0:
		return cmp.Compare(strings.ToLower(a.Process), strings.ToLower(b.Process))
	case 1:
		return cmp.Compare(a.PID, b.PID)
	case 2:
		return cmp.Compare(a.Family, b.Family)
	case 3:
		return cmp.Compare(a.Protocol, b.Protocol)
	case 4:
		return a.Local.Compare(b.Local)
	case 5:
		return a.Peer.Compare(b.Peer)
	default:
		return cmp.Compare(a.State, b.State)
	}
}

func (v *connectionsView) refreshRows() {
	needle := strings.TrimSpace(v.filter.Text)
	ip, _ := netip.ParseAddr(needle)
	needle = strings.ToLower(needle)
	v.visible = v.visible[:0]
	for _, row := range v.rows {
		switch v.kind.Selected {
		case "Connected":
			if row.Protocol != "TCP" || !row.HasPeer || !row.Peer.IP.IsValid() {
				continue
			}
		case "Listening":
			if row.Protocol != "TCP" || row.State != "LISTEN" {
				continue
			}
		case "UDP":
			if row.Protocol != "UDP" {
				continue
			}
		}
		if ip.IsValid() {
			if row.Local.IP != ip && (!row.HasPeer || row.Peer.IP != ip) {
				continue
			}
		} else if needle != "" && !strings.Contains(strings.ToLower(strings.Join(connectionCells(row), " ")), needle) {
			continue
		}
		v.visible = append(v.visible, row)
	}
	slices.SortStableFunc(v.visible, func(a, b macdiag.Connection) int {
		n := compareConnection(a, b, v.sortColumn)
		if v.reverse {
			return -n
		}
		return n
	})
	old := v.selected
	v.selected = nil
	v.table.UnselectAll()
	v.table.Refresh()
	if old != nil {
		if row := slices.Index(v.visible, *old); row >= 0 {
			v.table.Select(widget.TableCellID{Row: row, Col: 0})
		}
	}
	if !v.sample.IsZero() {
		v.count.SetText(fmt.Sprintf("%d of %d visible sockets · Sampled at %s", len(v.visible), len(v.rows), v.sample.Format(time.TimeOnly)))
	}
	v.refreshSelection()
}

func (u *ui) refreshConnectionInventory() {
	if v := u.connections.view; v != nil && v.selected != nil {
		v.refreshSelection()
	}
}

func (v *connectionsView) inventoryRow() int {
	u := v.owner
	s := u.currentSession
	if v.selected == nil || !v.selected.HasPeer || !v.selected.Peer.IP.Is4() ||
		u.scanning || s == nil || s.State != session.Complete || !s.Target.Contains(v.selected.Peer.IP) {
		return -1
	}
	for i, h := range u.sorted {
		if h.IP == v.selected.Peer.IP {
			return i
		}
	}
	return -1
}

func (v *connectionsView) refreshSelection() {
	v.jump.Disable()
	v.copy.Disable()
	v.inventory.SetText("")
	if v.selected == nil {
		v.details.SetText("Select a socket to inspect its endpoints.")
		return
	}
	row := *v.selected
	fields := connectionCells(row)
	v.details.SetText(fmt.Sprintf("%s (PID %d) · %s %s · %s · Local %s · Peer %s",
		fields[0], row.PID, row.Family, row.Protocol, fields[6], fields[4], fields[5]))
	v.copy.Enable()
	if v.inventoryRow() >= 0 {
		s := v.owner.currentSession
		v.inventory.SetText(fmt.Sprintf("Inventory match in %s · Scan finished %s", s.Target, s.FinishedAt.Local().Format(time.DateTime)))
		v.jump.Enable()
	}
}

func (v *connectionsView) showInInventory() {
	row := v.inventoryRow()
	if row < 0 {
		v.refreshSelection()
		return
	}
	u := v.owner
	cell := widget.TableCellID{Row: row, Col: 2}
	u.table.Select(cell)
	u.table.ScrollTo(cell)
	u.win.RequestFocus()
}

func (v *connectionsView) cancel() {
	if job := v.owner.connections.job; job != nil {
		job.cancel()
		v.stop.Disable()
		v.status.SetText("Stopping…")
	}
}

func (v *connectionsView) takeSnapshot() {
	c := &v.owner.connections
	if c.job != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &connectionJob{cancel: cancel, done: make(chan struct{})}
	c.job = job
	v.refresh.Disable()
	v.stop.Enable()
	v.status.SetText("Reading visible sockets…")
	v.progress.Show()
	v.progress.Start()
	go func() {
		result, err := macdiag.ReadConnections(ctx)
		close(job.done)
		fyne.Do(func() {
			c.job = nil
			defer cancel()
			if c.view != v {
				if c.view != nil {
					c.view.refresh.Enable()
					c.view.status.SetText("Ready · Select Refresh to sample.")
				}
				return
			}
			v.progress.Stop()
			v.progress.Hide()
			v.refresh.Enable()
			v.stop.Disable()
			v.showSnapshot(result, err, ctx.Err() != nil)
		})
	}()
}

func (v *connectionsView) showSnapshot(result macdiag.Snapshot, err error, cancelled bool) {
	stale := ""
	if !v.sample.IsZero() {
		stale = " Previous sample retained."
	}
	switch {
	case cancelled:
		v.status.SetText("Cancelled." + stale)
	case errors.Is(err, context.DeadlineExceeded):
		v.status.SetText("Timed out reading visible sockets." + stale)
	case errors.Is(err, macdiag.ErrOutputLimit):
		v.status.SetText("Socket output exceeded the display limit." + stale)
	case errors.Is(err, os.ErrNotExist):
		v.status.SetText("lsof is unavailable on this Mac." + stale)
	case err != nil:
		v.status.SetText("Could not read visible sockets." + stale)
	case result.Partial && !v.sample.IsZero():
		v.status.SetText("Partial refresh." + stale)
	default:
		v.rows, v.sample = result.Rows, time.Now()
		v.refreshRows()
		switch {
		case result.Partial:
			v.status.SetText("Partial sample · Some socket records were unavailable.")
		case len(v.rows) == 0:
			v.status.SetText("No visible matches.")
		default:
			v.status.SetText("Complete")
		}
	}
}

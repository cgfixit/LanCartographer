// Package gui is LanCartographer's Fyne interface: toolbar, live scan
// table, side panel, context menus, favorites, export, and the 2D map
// placeholder. Widget code stays thin; every launch or formatting
// decision lives in actions.go as a pure, unit-tested helper.
package gui

import (
	"context"
	"fmt"
	"image/color"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/authz"
	"github.com/local/lancartographer/internal/discover"
	"github.com/local/lancartographer/internal/export"
	"github.com/local/lancartographer/internal/favorites"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
	"github.com/local/lancartographer/internal/winutil"
	"github.com/local/lancartographer/internal/wol"
)

// Version is shown in the About box.
const Version = "0.1.0"

// guiDeps binds the outside world so widget code stays decision-free.
type guiDeps struct {
	localSubnets func(includeVirtual bool) ([]discover.InterfaceInfo, error)
	isElevated   func() bool
}

// ui carries the window state the toolbar, table, panel, and menus share.
// All fields are touched on the UI thread only; engine goroutines deliver
// updates through fyne.Do.
type ui struct {
	app    fyne.App
	win    fyne.Window
	store  *hostStore
	sorted []model.Host // snapshot behind the table, ordered per sort state
	local  []netip.Prefix

	topBar    *fyne.Container
	subnet    *widget.SelectEntry
	scanBtn   *widget.Button
	stopBtn   *widget.Button
	mapBtn    *widget.Button
	elevateBtn *widget.Button

	progress *widget.ProgressBar
	split    *container.Split
	table    *widget.Table
	panel    *fyne.Container
	mapView  *fyne.Container

	statusBar  *fyne.Container
	statusHosts *widget.Label
	statusPorts *widget.Label
	statusShare *widget.Label
	statusElaps *widget.Label
	statusErr   *widget.Label

	// clickable table header buttons (index-aligned with tableColumns)
	headerBtns []*widget.Button

	// scan state
	scanning bool
	cancel   context.CancelFunc
	started  time.Time
	tickStop chan struct{} // closed by finishScan to end the elapsed ticker goroutine

	// table sorting (column index into tableColumns)
	sortCol int
	sortAsc bool

	// last scanned prefix, for WOL broadcast derivation
	lastPrefix netip.Prefix

	selectedIP string

	deps guiDeps
}

// Run opens the LanCartographer window and blocks until it closes.
func Run() error {
	a := app.New()
	w := a.NewWindow("LanCartographer")
	w.Resize(fyne.NewSize(1120, 680))

	u := &ui{
		app:   a,
		win:   w,
		store: newHostStore(),
		deps: guiDeps{
			localSubnets: discover.LocalSubnets,
			isElevated:   winutil.IsElevated,
		},
		sortCol: 2,
		sortAsc: true,
	}

	u.captureLocalSubnets()
	u.progress = widget.NewProgressBar()
	u.buildTopBar()
	u.buildPanel()
	u.buildTable()
	u.buildMapView()
	u.buildStatusBar()
	u.refreshTable()

	stack := container.NewStack(u.split, u.mapView)
	u.mapView.Hide()
	u.win.SetContent(container.NewBorder(u.topBar, u.statusBar, nil, nil,
		container.NewVBox(u.progress, stack)))
	u.win.SetMainMenu(u.mainMenu())
	u.win.Show()
	u.maybeFirstRun()
	a.Run()
	return nil
}

// captureLocalSubnets records the local physical subnets once; they feed
// both the combo dropdown and the scan confirmation gate.
func (u *ui) captureLocalSubnets() {
	ifs, err := u.deps.localSubnets(false)
	if err != nil {
		return
	}
	for _, i := range ifs {
		u.local = append(u.local, i.Prefix.Masked())
	}
}

// buildTopBar lays out the editable subnet combo, Scan/Stop, favorites,
// export, the map toggle, and (when unelevated) the elevate button.
func (u *ui) buildTopBar() {
	u.subnet = widget.NewSelectEntry(nil)
	u.subnet.SetPlaceHolder("Subnet, e.g. 10.0.0.0/24")
	opts := make([]string, 0, len(u.local))
	for _, p := range u.local {
		opts = append(opts, p.String())
	}
	u.subnet.SetOptions(opts)
	if len(opts) > 0 {
		u.subnet.SetText(opts[0])
	}

	u.scanBtn = widget.NewButtonWithIcon("Scan", theme.MediaPlayIcon(), u.onScan)
	u.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), u.onStop)
	u.stopBtn.Disable()

	u.mapBtn = widget.NewButtonWithIcon("Map — v1.1", theme.GridIcon(), u.onMapToggle)

	items := []fyne.CanvasObject{
		u.subnet,
		u.scanBtn,
		u.stopBtn,
		widget.NewButtonWithIcon("Save Favorites", theme.FolderIcon(), u.onSaveFavorites),
		widget.NewButtonWithIcon("Load Favorites", theme.FolderOpenIcon(), u.onLoadFavorites),
		widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), u.onExport),
		u.mapBtn,
	}
	if !u.deps.isElevated() {
		u.elevateBtn = widget.NewButtonWithIcon("Elevate", theme.LoginIcon(), u.onElevate)
		items = append(items, u.elevateBtn)
	}
	u.topBar = container.NewHBox(items...)
}

// buildStatusBar creates the bottom strip: counts, elapsed, last error,
// and the standing authorization reminder.
func (u *ui) buildStatusBar() {
	u.statusHosts = widget.NewLabel("0 hosts")
	u.statusPorts = widget.NewLabel("0 open ports")
	u.statusShare = widget.NewLabel("0 shares")
	u.statusElaps = widget.NewLabel("0.0s")
	u.statusErr = widget.NewLabel("")
	reminder := widget.NewLabel(authz.Notice)
	reminder.Importance = widget.WarningImportance
	reminder.Truncation = fyne.TextTruncateEllipsis
	u.statusBar = container.NewHBox(
		u.statusHosts, widget.NewSeparator(), u.statusPorts, widget.NewSeparator(),
		u.statusShare, widget.NewSeparator(), u.statusElaps, widget.NewSeparator(),
		u.statusErr, layoutSpacer(), reminder)
}

// layoutSpacer returns an expanding empty object for the status bar.
func layoutSpacer() fyne.CanvasObject {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(20, 1))
	return spacer
}

// refreshStatus recomputes the status-bar counts from the store.
func (u *ui) refreshStatus() {
	hosts, openPorts, shares := u.store.counts()
	u.statusHosts.SetText(fmt.Sprintf("%d hosts", hosts))
	u.statusPorts.SetText(fmt.Sprintf("%d open ports", openPorts))
	u.statusShare.SetText(fmt.Sprintf("%d shares", shares))
}

// refreshTable re-sorts the store snapshot behind the table and repaints.
func (u *ui) refreshTable() {
	u.sorted = sortHosts(u.store.snapshot(), u.sortCol, u.sortAsc)
	u.table.Refresh()
	u.refreshStatus()
}

// mainMenu is the window menu bar: File (export, favorites, quit) and Help
// (About).
func (u *ui) mainMenu() *fyne.MainMenu {
	return fyne.NewMainMenu(
		fyne.NewMenu("File",
			fyne.NewMenuItem("Export…", u.onExport),
			fyne.NewMenuItem("Save Favorites", u.onSaveFavorites),
			fyne.NewMenuItem("Load Favorites", u.onLoadFavorites),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", func() { u.app.Quit() }),
		),
		fyne.NewMenu("Help",
			fyne.NewMenuItem("About LanCartographer", func() {
				dialog.ShowInformation("About LanCartographer", aboutText(), u.win)
			}),
		),
	)
}

// aboutText is the About box body: version, license, authorization notice,
// and build target.
func aboutText() string {
	return fmt.Sprintf("LanCartographer %s — LAN inventory for Windows.\n\n"+
		"MIT License.\n\n%s\n\nBuilt for Windows 10/11 (amd64).", Version, authz.Notice)
}

// firstRunPath is the %AppData% marker recording that the operator has seen
// the authorization notice.
func firstRunPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "LanCartographer", "firstrun_seen")
}

// maybeFirstRun shows the authorization dialog on first launch; the "I
// understand" button persists the marker so it never appears again.
func (u *ui) maybeFirstRun() {
	if _, err := os.Stat(firstRunPath()); err == nil {
		return
	}
	label := widget.NewLabel(authz.Notice)
	label.Wrapping = fyne.TextWrapWord
	var popup *widget.PopUp
	understand := widget.NewButton("I understand", func() {
		if err := os.MkdirAll(filepath.Dir(firstRunPath()), 0o755); err == nil {
			_ = os.WriteFile(firstRunPath(), []byte(time.Now().Format(time.RFC3339)), 0o644)
		}
		popup.Hide()
	})
	popup = widget.NewModalPopUp(container.NewVBox(label, understand), u.win.Canvas())
	popup.Show()
}

// onMapToggle swaps the table split for the map placeholder and back. The
// table never depends on the map; v1.1 ships the real 2D ring.
func (u *ui) onMapToggle() {
	if u.mapView.Visible() {
		u.mapView.Hide()
		u.split.Show()
		u.mapBtn.SetText("Map — v1.1")
		return
	}
	u.split.Hide()
	u.mapView.Show()
	u.mapBtn.SetText("Table")
}

// onScan parses the subnet field and runs the confirmation gate before
// starting the engine.
func (u *ui) onScan() {
	u.statusErr.SetText("")
	p, err := netip.ParsePrefix(strings.TrimSpace(u.subnet.Text))
	if err != nil {
		u.statusErr.SetText("invalid subnet: " + err.Error())
		return
	}
	target := p.Masked()
	if authz.NeedsConfirmation(target, u.local) {
		dialog.ShowConfirm("Confirm scan target",
			authz.Notice+"\n\nYou asked to scan "+target.String()+", which is outside your local subnets, not private space, or broader than a /22.\n\nScan anyway?",
			func(ok bool) {
				if ok {
					u.startScan(target)
				}
			}, u.win)
		return
	}
	u.startScan(target)
}

// startScan resets the inventory and launches the engine on a goroutine.
// Events pump back onto the UI thread through fyne.Do.
func (u *ui) startScan(target netip.Prefix) {
	if u.scanning {
		return
	}
	u.store = newHostStore()
	u.selectedIP = ""
	u.lastPrefix = target
	u.scanning = true
	u.started = time.Now()
	u.scanBtn.Disable()
	u.stopBtn.Enable()
	u.progress.SetValue(0)
	u.showPanelPlaceholder()
	u.refreshTable()

	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	u.tickStop = make(chan struct{})
	stop := u.tickStop
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(u.started).Round(time.Second)
				fyne.Do(func() { u.statusElaps.SetText(elapsed.String()) })
			case <-stop:
				return
			}
		}
	}()

	opts := probe.Options{
		Ports:        defaultPorts(),
		DialTimeout:  400 * time.Millisecond,
		ProbeTimeout: 800 * time.Millisecond,
		Concurrency:  128,
	}
	out := make(chan scan.Event, 64)
	go func() {
		defer close(out)
		_ = scan.Run(ctx, target, opts, out)
	}()
	go func() {
		for ev := range out {
			ev := ev
			fyne.Do(func() { u.applyEvent(ev) })
		}
		fyne.Do(u.finishScan)
	}()
}

// applyEvent folds one engine event into the UI state.
func (u *ui) applyEvent(ev scan.Event) {
	switch ev.Kind {
	case scan.EventHostFound, scan.EventPortFound, scan.EventShareFound:
		u.store.apply(ev)
		u.refreshTable()
		if u.selectedIP == ev.Host.IP.String() {
			u.showSelectedHost()
		}
	case scan.EventProgress:
		u.progress.SetValue(ev.Progress)
	case scan.EventError:
		if ev.Err != nil {
			u.statusErr.SetText(ev.Err.Error())
		}
	case scan.EventDone:
		u.progress.SetValue(1)
	}
}

// finishScan runs on the UI thread once the engine's channel has closed.
func (u *ui) finishScan() {
	if u.tickStop != nil {
		close(u.tickStop) // ends the elapsed-ticker goroutine
		u.tickStop = nil
	}
	u.scanning = false
	u.cancel = nil
	u.scanBtn.Enable()
	u.stopBtn.Disable()
	u.statusElaps.SetText(time.Since(u.started).Round(time.Millisecond).String())
}

// onStop cancels the scan context; the engine unwinds in under a second.
func (u *ui) onStop() {
	if u.cancel != nil {
		u.cancel()
	}
}

// onSaveFavorites persists the current inventory as favorites.
func (u *ui) onSaveFavorites() {
	if err := favorites.Save(u.store.snapshot()); err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.statusErr.SetText("")
}

// onLoadFavorites loads saved favorites into the table WITHOUT scanning.
func (u *ui) onLoadFavorites() {
	hosts, err := favorites.Load()
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.store.set(hosts)
	u.selectedIP = ""
	u.showPanelPlaceholder()
	u.refreshTable()
}

// onExport asks for a path (with .csv/.json filters) and writes the
// current inventory through the export package.
func (u *ui) onExport() {
	fd := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if wr == nil {
			return // cancelled
		}
		path := wr.URI().Path()
		wr.Close()
		format := "csv"
		if strings.EqualFold(filepath.Ext(path), ".json") {
			format = "json"
		}
		if err := export.ToFile(path, u.store.snapshot(), format); err != nil {
			dialog.ShowError(err, u.win)
		}
	}, u.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".csv", ".json"}))
	fd.SetFileName("inventory.csv")
	fd.Show()
}

// onElevate relaunches the same executable elevated and exits this
// instance so only one window remains.
func (u *ui) onElevate() {
	if err := winutil.RelaunchElevated(); err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.app.Quit()
}

// showSelectedHost repaints the side panel for the current selection.
func (u *ui) showSelectedHost() {
	h, ok := u.store.get(u.selectedIP)
	if !ok {
		u.showPanelPlaceholder()
		return
	}
	u.showHost(h)
}

// sendWOL fires one magic packet at the host on a goroutine, bounded by a
// 3s context; errors land in the status bar.
func (u *ui) sendWOL(h model.Host) {
	mac, err := parseMAC(h.MAC)
	if err != nil {
		return
	}
	pkt, err := wol.MagicPacket(mac)
	if err != nil {
		return
	}
	broadcast := "255.255.255.255:9"
	if u.lastPrefix.IsValid() {
		broadcast = broadcastAddr(u.lastPrefix)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := wol.Send(ctx, pkt, broadcast); err != nil {
			fyne.Do(func() { u.statusErr.SetText("Wake-on-LAN: " + err.Error()) })
		}
	}()
}

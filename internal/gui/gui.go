//go:build cgo

// Package gui is LanCartographer's Fyne interface: toolbar, live scan
// table, side panel, context menus, favorites, and export. Widget code
// stays thin; every launch or formatting decision lives in actions.go
// as a pure, unit-tested helper.
package gui

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
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

	"github.com/local/lancartographer/internal/appdata"
	"github.com/local/lancartographer/internal/authz"
	"github.com/local/lancartographer/internal/discover"
	"github.com/local/lancartographer/internal/export"
	"github.com/local/lancartographer/internal/favorites"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
	"github.com/local/lancartographer/internal/session"
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

	topBar     *fyne.Container
	subnet     *widget.SelectEntry
	scanBtn    *widget.Button
	stopBtn    *widget.Button
	themeBtn   *widget.Button
	elevateBtn *widget.Button

	progress *widget.ProgressBar
	split    *container.Split
	table    *widget.Table
	panel    *fyne.Container

	statusBar   *fyne.Container
	statusHosts *widget.Label
	statusPorts *widget.Label
	statusShare *widget.Label
	statusElaps *widget.Label
	statusErr   *widget.Label
	statusScan  *widget.Label

	// status-bar rule rectangles, live-recolored by applyTheme
	statusSeparators []*canvas.Rectangle

	// empty-table guidance layer sharing the body stack with the table
	emptyHint  *fyne.Container
	emptyTitle *widget.Label
	emptySub   *widget.Label

	// clickable table header buttons (index-aligned with tableColumns)
	headerBtns []*widget.Button

	// scan state
	scanning       bool
	cancel         context.CancelFunc
	started        time.Time
	tickStop       chan struct{} // closed by finishScan to end the elapsed ticker goroutine
	currentSession *session.Session
	scanDone       bool

	qualityView *qualityView
	qualityJob  *qualityJob

	// table repaint coalescing: engine events fold into the store per event,
	// but the sort+repaint is throttled so a fast scan costs O(events) store
	// updates, not O(events x n log n) re-sorts. finishScan flushes.
	lastRefresh  time.Time
	refreshDirty bool
	throttle     time.Duration
	repaints     int // test hook: counts full refreshTable invocations

	// table sorting (column index into tableColumns)
	sortCol int
	sortAsc bool

	// last scanned prefix, for WOL broadcast derivation
	lastPrefix netip.Prefix

	selectedIP string

	settings AppSettings

	deps guiDeps
}

// Run opens the LanCartographer window and blocks until it closes.
func Run() error {
	a := app.NewWithID("com.cgfixit.lancartographer")
	settings, _ := LoadSettings()
	if settings.Theme == "dark" {
		a.Settings().SetTheme(theme.DarkTheme())
	} else {
		a.Settings().SetTheme(theme.LightTheme())
	}
	w := a.NewWindow("LanCartographer")
	w.Resize(fyne.NewSize(1120, 680))

	u := newUI(a, w, guiDeps{
		localSubnets: discover.LocalSubnets,
		isElevated:   winutil.IsElevated,
	})
	u.applyTheme(u.settings.Theme)

	u.win.Show()
	u.maybeFirstRun()
	a.Run()
	if job := u.qualityJob; job != nil {
		job.cancel()
		<-job.done
	}
	return nil
}

// newUI builds the full window content without showing it or running the
// event loop, so tests can construct and drive the UI with fyne's test
// harness. Behavior is identical to what Run() used to inline.
func newUI(a fyne.App, w fyne.Window, deps guiDeps) *ui {
	u := &ui{
		app:     a,
		win:     w,
		store:   newHostStore(),
		deps:    deps,
		sortCol: 2,
		sortAsc: true,

		throttle: 120 * time.Millisecond,
	}

	u.captureLocalSubnets()
	u.settings, _ = LoadSettings()
	u.progress = widget.NewProgressBar()
	u.buildTopBar()
	u.buildPanel()
	u.buildTable()
	u.buildStatusBar()
	u.refreshTable()

	u.win.SetContent(container.NewBorder(
		container.NewVBox(u.topBar, u.progress),
		u.statusBar, nil, nil,
		u.split))
	u.win.SetMainMenu(u.mainMenu())
	return u
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
// export, theme, and (when unelevated) the elevate button.
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
	u.scanBtn.Importance = widget.HighImportance
	u.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), u.onStop)
	u.stopBtn.Importance = widget.DangerImportance
	u.stopBtn.Disable()

	u.themeBtn = widget.NewButtonWithIcon(themeModeLabel(u.settings.Theme), theme.ColorPaletteIcon(), u.onThemeToggle)

	items := []fyne.CanvasObject{
		container.NewGridWrap(fyne.NewSize(230, u.subnet.MinSize().Height), u.subnet),
		u.scanBtn,
		u.stopBtn,
		u.themeBtn,
		widget.NewButtonWithIcon("Save", theme.FolderIcon(), u.onSaveFavorites),
		widget.NewButtonWithIcon("Load", theme.FolderOpenIcon(), u.onLoadFavorites),
		widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), u.onExport),
		widget.NewButtonWithIcon("Sessions", theme.HistoryIcon(), u.onSessions),
	}
	if runtime.GOOS == "windows" && !u.deps.isElevated() {
		u.elevateBtn = widget.NewButtonWithIcon("Elevate", theme.LoginIcon(), u.onElevate)
		items = append(items, u.elevateBtn)
	}
	u.topBar = container.NewHBox(items...)
}

// statusReminder is the short standing authorization reminder shown in the
// status bar; the full authz.Notice stays in first-run, About, and the scan
// confirmation dialog.
const statusReminder = "Scan only networks you own or have authorization to inventory."

// buildStatusBar creates the bottom strip: counts separated by thin rules
// on the left, and on the right a scanning indicator plus the standing
// authorization reminder.
func (u *ui) buildStatusBar() {
	u.statusHosts = widget.NewLabel("0 hosts")
	u.statusPorts = widget.NewLabel("0 open ports")
	u.statusShare = widget.NewLabel("0 shares")
	u.statusElaps = widget.NewLabel("0.0s")
	u.statusErr = widget.NewLabel("")
	u.statusScan = widget.NewLabel("Scanning…")
	u.statusScan.Importance = widget.WarningImportance
	u.statusScan.Hide()
	reminder := widget.NewLabel(statusReminder)
	reminder.Importance = widget.WarningImportance
	reminder.Alignment = fyne.TextAlignTrailing
	// Measure the natural size BEFORE enabling truncation: an
	// ellipsis-truncated label reports a collapsed MinSize, which is what
	// shrunk the Border right slot to "…" at every window width. The
	// GridWrap pins the full-sentence width as the minimum; on very narrow
	// windows the label still ellipsizes within it.
	reminderMin := reminder.MinSize()
	reminder.Truncation = fyne.TextTruncateEllipsis
	u.statusSeparators = nil
	counters := container.NewHBox(
		u.statusHosts, u.thinSeparator(), u.statusPorts, u.thinSeparator(),
		u.statusShare, u.thinSeparator(), u.statusElaps, u.thinSeparator(),
		u.statusErr)
	u.statusBar = container.NewBorder(nil, nil, counters,
		container.NewHBox(u.statusScan, container.NewGridWrap(reminderMin, reminder)))
}

// statusSeparators tracks the live status-bar rule rectangles so applyTheme
// can re-resolve their fill: canvas colors are captured at build time, so a
// theme toggle would otherwise leave the 1px rules in the old theme's color.
// It is per-ui state (not package state) so repeated window builds in one
// process, including tests, do not accumulate stale rectangles.

// thinSeparator returns a 1px-wide vertical rule for the status bar; a
// widget.Separator in an HBox lays out horizontally, which reads wrong.
func (u *ui) thinSeparator() fyne.CanvasObject {
	rule := canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
	u.statusSeparators = append(u.statusSeparators, rule)
	return container.NewGridWrap(fyne.NewSize(1, 16), rule)
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
	u.updateEmptyHint()
	u.lastRefresh = time.Now()
	u.refreshDirty = false
	u.repaints++
}

// maybeRefreshTable coalesces repaint requests during a scan: within the
// throttle window it marks the table dirty (flushed by the next full
// refresh, and always by finishScan) but still updates the empty-table
// hint, which is cheap and must track store emptiness exactly.
func (u *ui) maybeRefreshTable() {
	if u.throttle > 0 && time.Since(u.lastRefresh) < u.throttle {
		u.refreshDirty = true
		u.updateEmptyHint()
		return
	}
	u.refreshTable()
}

// mainMenu adds the platform-specific Tools menu on macOS.
func (u *ui) mainMenu() *fyne.MainMenu {
	file := fyne.NewMenu("File",
		fyne.NewMenuItem("Sessions…", u.onSessions),
		fyne.NewMenuItem("Export…", u.onExport),
		fyne.NewMenuItem("Save Favorites", u.onSaveFavorites),
		fyne.NewMenuItem("Load Favorites", u.onLoadFavorites),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() { u.app.Quit() }),
	)
	help := fyne.NewMenu("Help",
		fyne.NewMenuItem("About LanCartographer", func() {
			dialog.ShowInformation("About LanCartographer", aboutText(), u.win)
		}),
	)
	if runtime.GOOS == "darwin" {
		tools := fyne.NewMenu("Tools", fyne.NewMenuItem("Internet Quality…", u.showInternetQuality))
		return fyne.NewMainMenu(file, tools, help)
	}
	return fyne.NewMainMenu(file, help)
}

// aboutText is the About box body: version, license, authorization notice,
// and build target.
func aboutText() string {
	return fmt.Sprintf("LanCartographer %s — LAN inventory.\n\n"+
		"MIT License.\n\n%s\n\nBuilt for %s/%s.", Version, authz.Notice, runtime.GOOS, runtime.GOARCH)
}

// firstRunPath is the per-user marker recording that the operator has seen
// the authorization notice.
func firstRunPath() (string, error) {
	return appdata.Path("firstrun_seen")
}

// maybeFirstRun shows the authorization dialog on first launch; the "I
// understand" button persists the marker so it never appears again.
func (u *ui) maybeFirstRun() {
	p, pathErr := firstRunPath()
	if pathErr == nil {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	label := widget.NewLabel(authz.Notice)
	label.Wrapping = fyne.TextWrapWord
	var popup *widget.PopUp
	understand := widget.NewButton("I understand", func() {
		if pathErr == nil {
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err == nil {
				_ = os.WriteFile(p, []byte(time.Now().Format(time.RFC3339)), 0o600)
			}
		}
		popup.Hide()
	})
	popup = widget.NewModalPopUp(container.NewVBox(label, understand), u.win.Canvas())
	popup.Resize(fyne.NewSize(600, 180))
	popup.Show()
}

// applyTheme switches the app-wide theme; anything but "dark" means light.
// Status-bar separators captured their color at build time, so re-resolve
// them here after the switch.
func (u *ui) applyTheme(mode string) {
	if mode == "dark" {
		fyne.CurrentApp().Settings().SetTheme(theme.DarkTheme())
	} else {
		fyne.CurrentApp().Settings().SetTheme(theme.LightTheme())
	}
	for _, rule := range u.statusSeparators {
		rule.FillColor = theme.Color(theme.ColorNameSeparator)
		rule.Refresh()
	}
}

// onThemeToggle flips between the light and dark themes and persists the
// choice; a save failure is reported in the status bar, never fatal.
func (u *ui) onThemeToggle() {
	if u.settings.Theme == "dark" {
		u.settings.Theme = "light"
	} else {
		u.settings.Theme = "dark"
	}
	u.applyTheme(u.settings.Theme)
	u.themeBtn.SetText(themeModeLabel(u.settings.Theme))
	if err := SaveSettings(u.settings); err != nil {
		u.statusErr.SetText("settings: " + err.Error())
	}
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
	if job := u.qualityJob; job != nil {
		if !job.scanAfter.IsValid() {
			job.scanAfter = target
			u.scanBtn.Disable()
			u.cancelInternetQuality()
		}
		return
	}
	u.table.UnselectAll()
	u.store = newHostStore()
	u.selectedIP = ""
	u.lastPrefix = target
	u.scanning = true
	u.refreshQualityControls()
	u.started = time.Now()
	u.scanBtn.Disable()
	u.stopBtn.Enable()
	u.statusScan.Show()
	u.progress.SetValue(0)
	u.showPanelPlaceholder()
	u.refreshTable()

	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	u.tickStop = make(chan struct{})
	stop := u.tickStop
	started := u.started
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(started).Round(time.Second)
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
	snapshot := session.New(target, opts, Version)
	snapshot.StartedAt = u.started.UTC()
	u.currentSession = &snapshot
	u.scanDone = false
	out := make(chan scan.Event, 64)
	result := make(chan error, 1)
	go func() {
		result <- scan.Run(ctx, target, opts, out)
		close(out)
	}()
	go func() {
		for ev := range out {
			ev := ev
			fyne.Do(func() { u.applyEvent(ev) })
		}
		err := <-result
		fyne.Do(func() {
			u.endSession(err)
			u.finishScan()
		})
	}()
}

// applyEvent folds one engine event into the UI state. Store updates apply
// per event (correctness-critical); repaints are throttled except for the
// terminal events, which always flush so the settled state is exact.
func (u *ui) applyEvent(ev scan.Event) {
	switch ev.Kind {
	case scan.EventHostFound, scan.EventPortFound, scan.EventShareFound:
		u.store.apply(ev)
		u.maybeRefreshTable()
		if u.selectedIP == ev.Host.IP.String() {
			u.showSelectedHost()
		}
	case scan.EventProgress:
		u.progress.SetValue(ev.Progress)
		if u.currentSession != nil {
			u.currentSession.Progress = ev.Progress
		}
	case scan.EventError:
		if ev.Err != nil {
			u.statusErr.SetText(ev.Err.Error())
		}
		u.refreshTable()
	case scan.EventDone:
		u.scanDone = true
		if u.currentSession != nil {
			u.currentSession.Progress = 1
		}
		u.progress.SetValue(1)
		u.refreshTable()
	}
}

// finishScan runs on the UI thread once the engine's channel has closed.
func (u *ui) finishScan() {
	if u.tickStop != nil {
		close(u.tickStop) // ends the elapsed-ticker goroutine
		u.tickStop = nil
	}
	u.scanning = false
	u.refreshQualityControls()
	u.cancel = nil
	u.scanBtn.Enable()
	u.stopBtn.Disable()
	u.statusScan.Hide()
	u.statusElaps.SetText(time.Since(u.started).Round(time.Millisecond).String())
	u.refreshTable() // final flush: settled state is exact even if throttled
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
	if u.scanning {
		dialog.ShowInformation("Scan in progress", "Stop the scan before replacing the inventory.", u.win)
		return
	}
	hosts, err := favorites.Load()
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.table.UnselectAll()
	u.store.set(hosts)
	u.currentSession = nil // favorites carry no scan coverage or completion metadata
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

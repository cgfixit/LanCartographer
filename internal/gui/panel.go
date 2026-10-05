package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
)

// buildPanel creates the right-hand side panel container. It is rebuilt on
// every selection change; it is a plain pane, never modal.
func (u *ui) buildPanel() {
	u.panel = container.NewVBox()
	u.showPanelPlaceholder()
}

// buildMapView creates the v1.1 map placeholder shown by the toggle. The
// real 2D ring (maplayout.Layout + canvas nodes + click-to-select) ships
// in v1.1; the table must never depend on it.
func (u *ui) buildMapView() {
	u.mapView = container.NewCenter(container.NewVBox(
		widget.NewLabelWithStyle("Network Map", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("The 2D network map ships in v1.1."),
		widget.NewLabel("The table view is the complete inventory."),
	))
}

// showPanelPlaceholder resets the panel to its idle hint. The label goes
// straight into the VBox (no NewCenter) so it stretches to the panel width
// and truncation only engages when the panel is genuinely narrow.
func (u *ui) showPanelPlaceholder() {
	hint := widget.NewLabel("Select a host to see details.")
	hint.Truncation = fyne.TextTruncateEllipsis
	u.panel.Objects = []fyne.CanvasObject{hint}
	u.panel.Refresh()
}

// showHost fills the panel with the selected host's identity, actions,
// ports, and shares. Every child is wrapped in a tapBox so right-click
// offers the same menu as the table row.
func (u *ui) showHost(h model.Host) {
	menu := func(obj fyne.CanvasObject) fyne.CanvasObject {
		return u.wrapHostMenu(obj, h)
	}

	title := h.Name
	if title == "" {
		title = h.IP.String()
	}

	// Identity section.
	ident := container.NewGridWithColumns(2,
		menu(panelKV("IP", h.IP.String())),
		menu(panelKV("MAC", h.MAC)),
		menu(panelKV("Vendor", h.Vendor)),
		menu(panelKV("Found via", h.FoundVia)),
	)

	// Action buttons, enabled per the host's actual surface.
	actions := container.NewHBox(
		widget.NewButtonWithIcon("Explorer", theme.FolderIcon(), func() {
			u.launch(explorerArgv(explorerPath(h.IP, "")))
		}),
		widget.NewButtonWithIcon("Ping", theme.MediaPlayIcon(), func() {
			u.launch(pingArgv(h.IP))
		}),
	)
	if scheme := browserScheme(h); scheme != "" {
		scheme := scheme
		actions.Add(widget.NewButtonWithIcon("Browser", theme.GridIcon(), func() {
			u.launch(browserArgv(h.IP, scheme))
		}))
	}
	if h.HasRDP() {
		actions.Add(widget.NewButtonWithIcon("RDP", theme.DesktopIcon(), func() {
			u.launch(rdpArgv(h.IP))
		}))
	}
	if canWOL(h) {
		actions.Add(widget.NewButtonWithIcon("Wake", theme.ViewRefreshIcon(), func() {
			u.confirmWOL(h)
		}))
	}

	// Ports section: one row per probed port, open rows badged.
	ports := container.NewVBox(widget.NewLabelWithStyle("Ports", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if len(h.Ports) == 0 {
		ports.Add(widget.NewLabel("No ports probed yet."))
	} else {
		for _, p := range h.Ports {
			ports.Add(menu(u.portRow(h, p)))
		}
	}

	// Shares section: never lists files; denial records show their remark.
	shares := container.NewVBox(widget.NewLabelWithStyle("Shares", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	switch {
	case len(h.Shares) == 0:
		shares.Add(widget.NewLabel("None (445 closed or access denied)."))
	default:
		for _, s := range h.Shares {
			shares.Add(u.shareRow(h, s))
		}
	}

	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Truncation = fyne.TextTruncateEllipsis

	u.panel.Objects = []fyne.CanvasObject{
		titleLabel,
		widget.NewSeparator(),
		ident,
		actions,
		widget.NewSeparator(),
		ports,
		widget.NewSeparator(),
		shares,
		layout.NewSpacer(),
	}
	u.panel.Refresh()
}

// panelKV renders a key/value identity pair; the value is right-clickable.
func panelKV(key, value string) fyne.CanvasObject {
	k := widget.NewLabelWithStyle(key, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v := widget.NewLabel(value)
	v.Truncation = fyne.TextTruncateEllipsis
	return container.NewHBox(k, v)
}

// portRow renders one probed port: state, number, service, HTTP title.
// Open rows carry a badge: globe for web, folder for SMB, desktop for RDP.
func (u *ui) portRow(h model.Host, p model.Port) fyne.CanvasObject {
	state := widget.NewLabel(p.State)
	if p.State == model.PortOpen {
		state.Importance = widget.SuccessImportance
	}
	text := fmt.Sprintf("%d  %s", p.Number, p.Service)
	if p.Title != "" {
		text += "  —  " + p.Title
	}
	row := container.NewHBox(state, widget.NewLabel(text))
	if p.State != model.PortOpen {
		return row
	}
	switch p.Number {
	case 80, 443, 8080, 8443:
		row.Add(widget.NewIcon(theme.GridIcon()))
	case 445:
		row.Add(widget.NewIcon(theme.FolderIcon()))
	case 3389:
		row.Add(widget.NewIcon(theme.DesktopIcon()))
	}
	return row
}

// shareRow renders one share — name, type, remark, UNC — with an "Open in
// Explorer" button. The denial record renders its remark instead. Files are
// never listed.
func (u *ui) shareRow(h model.Host, s model.Share) fyne.CanvasObject {
	var text string
	if isDenialShare(s) {
		text = s.Remark
	} else {
		text = fmt.Sprintf("%s  (%s)", s.Name, s.Type)
		if s.Remark != "" {
			text += "  —  " + s.Remark
		}
		if s.UNC != "" {
			text += "  " + s.UNC
		}
	}
	row := container.NewHBox(
		widget.NewLabel(text),
		widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), func() {
			u.launch(explorerArgv(explorerPath(h.IP, s.Name)))
		}),
	)
	return u.wrapShareMenu(row, h, s)
}

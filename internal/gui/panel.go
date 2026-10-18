//go:build cgo

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

	// Identity section: right-aligned label column, left-aligned values.
	ident := container.NewVBox(
		menu(panelKV("IP", h.IP.String())),
		menu(panelKV("MAC", h.MAC)),
		menu(panelKV("Vendor", h.Vendor)),
		menu(panelKV("Found via", h.FoundVia)),
	)

	// Action buttons, enabled per the host's actual surface.
	actions := container.NewHBox(
		widget.NewButtonWithIcon(fileManagerName, theme.FolderIcon(), func() {
			u.launch(explorerArgv(explorerPath(h.IP, "")))
		}),
		widget.NewButtonWithIcon("Ping", theme.MediaPlayIcon(), func() {
			u.launch(pingArgv(h.IP))
		}),
	)
	if target := browserURL(h); target != "" {
		target := target
		actions.Add(widget.NewButtonWithIcon("Browser", theme.GridIcon(), func() {
			u.launch(browserArgv(target))
		}))
	}
	if argv := rdpArgv(h.IP); h.HasRDP() && argv != nil {
		actions.Add(widget.NewButtonWithIcon("RDP", theme.DesktopIcon(), func() {
			u.launch(argv)
		}))
	}
	if canWOL(h) {
		actions.Add(widget.NewButtonWithIcon("Wake", theme.ViewRefreshIcon(), func() {
			u.confirmWOL(h)
		}))
	}

	// Ports section: one row per probed port, open rows badged.
	ports := container.NewVBox(sectionHeader("Ports"))
	if len(h.Ports) == 0 {
		ports.Add(widget.NewLabel("No ports probed yet."))
	} else {
		for _, p := range h.Ports {
			ports.Add(menu(u.portRow(h, p)))
		}
	}

	// Shares section: never lists files; denial records show their remark.
	shares := container.NewVBox(sectionHeader("Shares"))
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
		sectionHeader("Identity"),
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

// sectionHeader uses a themed widget so switching themes recolors it.
func sectionHeader(text string) fyne.CanvasObject {
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

// panelKV renders a key/value identity pair on one row: the key sits in a
// fixed-width, right-aligned column so the values line up, and the value
// is right-clickable via the caller's menu wrapper.
func panelKV(key, value string) fyne.CanvasObject {
	k := widget.NewLabelWithStyle(key, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
	v := widget.NewLabel(value)
	v.Truncation = fyne.TextTruncateEllipsis
	return container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(90, k.MinSize().Height), k), nil, v)
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

// shareRow renders one share: a folder icon and bold name on the first
// line, type · remark on the second, the UNC smaller and dimmer, plus the
// "Open in "+fileManagerName action. The denial record renders its remark instead.
// Files are never listed.
func (u *ui) shareRow(h model.Host, s model.Share) fyne.CanvasObject {
	openBtn := widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), func() {
		u.launch(explorerArgv(explorerPath(h.IP, s.Name)))
	})
	if isDenialShare(s) {
		row := container.NewHBox(
			widget.NewIcon(theme.FolderIcon()),
			widget.NewLabel(s.Remark),
			openBtn,
		)
		return u.wrapShareMenu(row, h, s)
	}
	name := widget.NewLabelWithStyle(s.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	meta := s.Type
	if s.Remark != "" {
		meta += "  ·  " + s.Remark
	}
	lines := container.NewVBox(
		container.NewHBox(widget.NewIcon(theme.FolderIcon()), name),
		widget.NewLabel(meta),
	)
	if s.UNC != "" {
		unc := widget.NewLabel(s.UNC)
		unc.Importance = widget.LowImportance
		lines.Add(unc)
	}
	row := container.NewHBox(lines, openBtn)
	return u.wrapShareMenu(row, h, s)
}

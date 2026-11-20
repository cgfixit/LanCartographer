//go:build cgo

package gui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/servicecheck"
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
	tip := widget.NewLabel("Right-click a host for copy, open and check actions.")
	tip.Wrapping = fyne.TextWrapWord
	tip.TextStyle = fyne.TextStyle{Italic: true}
	u.panel.Objects = []fyne.CanvasObject{container.NewVBox(hint, tip)}
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
		menu(panelKV("Found via", foundViaLabel(h.FoundVia))),
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
	checkHost := widget.NewButtonWithIcon("Check service…", theme.SearchIcon(), func() {
		u.showServiceCheck(servicePrefillFor(h))
	})

	// Ports section: open ports first, each with a prefilled Check action;
	// closed and unanswered ports collapse into one dim line each.
	ports := container.NewVBox(u.portsSection(h, menu)...)

	// Shares section: never lists files; denial records show their remark.
	shares := container.NewVBox(sectionHeader("Shares"))
	switch {
	case len(h.Shares) == 0:
		none := widget.NewLabel("None found. Port 445 was closed, or the host did not allow listing.")
		none.Wrapping = fyne.TextWrapWord
		none.TextStyle = fyne.TextStyle{Italic: true}
		shares.Add(none)
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
		checkHost,
		widget.NewSeparator(),
		ports,
		widget.NewSeparator(),
		shares,
		layout.NewSpacer(),
	}
	u.panel.Refresh()
}

// portsSection lists open ports (with a Check action that only prefills
// Check Service) and summarizes the rest.
func (u *ui) portsSection(h model.Host, menu func(fyne.CanvasObject) fyne.CanvasObject) []fyne.CanvasObject {
	var open []model.Port
	var closed, silent []string
	for _, p := range h.Ports {
		switch p.State {
		case model.PortOpen:
			open = append(open, p)
		case model.PortTimeout:
			silent = append(silent, strconv.Itoa(p.Number))
		default:
			closed = append(closed, strconv.Itoa(p.Number))
		}
	}
	header := "Ports"
	if len(h.Ports) > 0 {
		header = fmt.Sprintf("Ports · %d open of %d probed", len(open), len(h.Ports))
	}
	out := []fyne.CanvasObject{sectionHeader(header)}
	if len(h.Ports) == 0 {
		return append(out, widget.NewLabel("No ports probed yet."))
	}
	if len(open) == 0 {
		none := widget.NewLabel("No open ports among those probed.")
		none.TextStyle = fyne.TextStyle{Italic: true}
		out = append(out, none)
	}
	sort.Slice(open, func(i, j int) bool { return open[i].Number < open[j].Number })
	for _, p := range open {
		out = append(out, menu(u.portRow(h, p)))
	}
	for _, group := range []struct {
		label string
		ports []string
	}{{"Closed", closed}, {"No answer", silent}} {
		if len(group.ports) == 0 {
			continue
		}
		line := widget.NewLabel(group.label + ": " + strings.Join(group.ports, ", "))
		line.Wrapping = fyne.TextWrapWord
		line.TextStyle = fyne.TextStyle{Italic: true}
		out = append(out, line)
	}
	return out
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
	if value == "" || value == emptyCell {
		v.SetText(emptyCell)
		v.Importance = widget.LowImportance
	}
	v.Truncation = fyne.TextTruncateEllipsis
	return container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(90, k.MinSize().Height), k), nil, v)
}

// portRow renders one open port: number, service and HTTP title, plus a
// Check action that prefills Check Service with this port. Prefilling
// contacts nothing; the check itself still asks for confirmation.
func (u *ui) portRow(h model.Host, p model.Port) fyne.CanvasObject {
	text := strconv.Itoa(p.Number)
	if p.Service != "" && p.Service != "unknown" {
		text += "  " + p.Service
	}
	if p.Title != "" {
		text += "  —  " + p.Title
	}
	if cert := p.CertSummary(time.Now()); cert != "" {
		text += "  ·  " + cert
	}
	label := widget.NewLabel(text)
	label.Truncation = fyne.TextTruncateEllipsis
	check := widget.NewButton("Check…", func() {
		protocol := servicecheck.SuggestProtocol(p.Number)
		if protocol == servicecheck.TCP && p.Service == "Web" {
			protocol = servicecheck.HTTP
		}
		u.showServiceCheck(&servicePrefill{host: h.IP.String(), port: p.Number, protocol: protocol})
	})
	check.Importance = widget.LowImportance
	return container.NewBorder(nil, nil, nil, check, label)
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

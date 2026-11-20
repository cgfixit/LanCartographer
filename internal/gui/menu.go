//go:build cgo

package gui

import (
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/authz"
	"github.com/local/lancartographer/internal/model"
)

// tapBox wraps a canvas object so it can carry primary (left-click) and
// secondary (right-click) tap handlers. The table cell template and every
// side-panel child use it. It is a real fyne widget (BaseWidget +
// CreateRenderer): a raw embedded fyne.CanvasObject has no renderer, so
// Fyne draws nothing.
type tapBox struct {
	widget.BaseWidget
	content     fyne.CanvasObject
	id          widget.TableCellID
	onPrimary   func()
	onSecondary func(*fyne.PointEvent)
}

// tapBox must satisfy both tap interfaces: the glfw driver records the
// deepest object matching ANY tap interface at mouse-press and only
// delivers the primary tap if that same object is fyne.Tappable at release.
var (
	_ fyne.Tappable          = (*tapBox)(nil)
	_ fyne.SecondaryTappable = (*tapBox)(nil)
)

// newTapBox wraps obj so TappedSecondary dispatches to onSecondary.
func newTapBox(obj fyne.CanvasObject) *tapBox {
	b := &tapBox{content: obj}
	b.ExtendBaseWidget(b)
	return b
}

// CreateRenderer renders just the wrapped content.
func (b *tapBox) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.content)
}

// MinSize delegates to the wrapped content.
func (b *tapBox) MinSize() fyne.Size { return b.content.MinSize() }

// Tapped implements fyne.Tappable (desktop left-click). Panel children
// leave onPrimary nil — they need no left-click action.
func (b *tapBox) Tapped(*fyne.PointEvent) {
	if b.onPrimary != nil {
		b.onPrimary()
	}
}

// TappedSecondary implements fyne.SecondaryTappable (desktop right-click).
func (b *tapBox) TappedSecondary(e *fyne.PointEvent) {
	if b.onSecondary != nil {
		b.onSecondary(e)
	}
}

// wrapHostMenu wraps a panel child so right-click shows the host menu.
func (u *ui) wrapHostMenu(obj fyne.CanvasObject, h model.Host) fyne.CanvasObject {
	box := newTapBox(obj)
	box.onSecondary = func(e *fyne.PointEvent) { u.showHostMenu(h, e) }
	return box
}

// wrapShareMenu wraps a share row so right-click shows the share menu.
func (u *ui) wrapShareMenu(obj fyne.CanvasObject, h model.Host, s model.Share) fyne.CanvasObject {
	box := newTapBox(obj)
	box.onSecondary = func(e *fyne.PointEvent) { u.showShareMenu(h, s, e) }
	return box
}

// showRowMenu is the table cell's right-click entry point.
func (u *ui) showRowMenu(id widget.TableCellID, e *fyne.PointEvent) {
	if id.Row >= len(u.sorted) {
		return
	}
	u.showHostMenu(u.sorted[id.Row], e)
}

// showHostMenu pops the per-host context menu at the event position.
func (u *ui) showHostMenu(h model.Host, e *fyne.PointEvent) {
	items := []*fyne.MenuItem{
		u.copyItem("Copy IP", h.IP.String()),
		u.copyItem("Copy MAC", h.MAC),
		u.copyItem("Copy name", h.Name),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Open in "+fileManagerName, func() {
			u.launch(explorerArgv(explorerPath(h.IP, "")))
		}),
	}
	if target := browserURL(h); target != "" {
		target := target
		items = append(items, fyne.NewMenuItem("Open in browser", func() {
			u.launch(browserArgv(target))
		}))
	}
	if argv := rdpArgv(h.IP); h.HasRDP() && argv != nil {
		items = append(items, fyne.NewMenuItem("Remote Desktop", func() {
			u.launch(argv)
		}))
	}
	items = append(items, fyne.NewMenuItem("Ping", func() {
		u.launch(pingArgv(h.IP))
	}))
	if canWOL(h) {
		items = append(items, fyne.NewMenuItem("Wake-on-LAN", func() {
			u.confirmWOL(h)
		}))
	}
	items = append(items, fyne.NewMenuItem("Check service…", func() {
		u.showServiceCheck(servicePrefillFor(h))
	}))
	items = append(items, u.connectionHostMenu(h.IP)...)
	u.popupMenu(fyne.NewMenu("Host", items...), e)
}

// showShareMenu pops the per-share context menu.
func (u *ui) showShareMenu(h model.Host, s model.Share, e *fyne.PointEvent) {
	items := []*fyne.MenuItem{
		u.copyItem("Copy UNC", s.UNC),
		u.copyItem("Copy name", s.Name),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Open in "+fileManagerName, func() {
			u.launch(explorerArgv(explorerPath(h.IP, s.Name)))
		}),
	}
	u.popupMenu(fyne.NewMenu("Share", items...), e)
}

// copyItem builds a menu item that copies text when the value is non-empty.
func (u *ui) copyItem(label, text string) *fyne.MenuItem {
	return fyne.NewMenuItem(label, func() {
		if text == "" {
			return
		}
		u.app.Clipboard().SetContent(text)
	})
}

// popupMenu shows menu at the absolute event position.
func (u *ui) popupMenu(menu *fyne.Menu, e *fyne.PointEvent) {
	widget.NewPopUpMenu(menu, u.win.Canvas()).ShowAtPosition(e.AbsolutePosition)
}

// launch starts argv[0] with argv[1:] as an ARGV LIST — never a shell
// string. Launch failures land in the status bar.
func (u *ui) launch(argv []string) {
	if len(argv) == 0 {
		return
	}
	if err := exec.Command(argv[0], argv[1:]...).Start(); err != nil {
		u.statusErr.SetText("launch: " + err.Error())
	}
}

// confirmWOL gates Wake-on-LAN behind a dialog restating the authorization
// notice plus the target MAC and broadcast address. OFF by default.
func (u *ui) confirmWOL(h model.Host) {
	broadcast := "255.255.255.255:9"
	if u.lastPrefix.IsValid() {
		broadcast = broadcastAddr(u.lastPrefix)
	}
	dialog.ShowConfirm("Wake-on-LAN",
		authz.Notice+"\n\nSend a magic packet to "+h.MAC+" via "+broadcast+"?",
		func(ok bool) {
			if ok {
				u.sendWOL(h)
			}
		}, u.win)
}

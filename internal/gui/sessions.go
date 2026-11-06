//go:build cgo

package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/appdata"
	"github.com/local/lancartographer/internal/session"
)

func (u *ui) endSession(err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		u.statusErr.SetText(err.Error())
	}
	if u.currentSession == nil {
		return
	}
	s := u.currentSession
	s.FinishedAt = time.Now().UTC()
	s.Hosts = u.store.snapshot()
	switch {
	case errors.Is(err, context.Canceled):
		s.State = session.Cancelled
	case err != nil:
		s.State = session.Failed
	case u.scanDone:
		s.State = session.Complete
	default:
		s.State = session.Incomplete
	}
	copy := s.Clone()
	u.currentSession = &copy
}

type sessionsView struct {
	owner                     *ui
	win                       fyne.Window
	before, after             *session.Session
	comparison                session.Comparison
	selected                  map[int]bool
	list                      *widget.List
	previewScroll             *container.Scroll
	summary, outcome, preview *widget.Label
	context, label, notes     *widget.Entry
	redact                    *widget.Check
	exportBtn, saveBtn        *widget.Button
}

func (u *ui) onSessions() { u.newSessionsView().win.Show() }

func (u *ui) newSessionsView() *sessionsView {
	v := &sessionsView{owner: u, win: u.app.NewWindow("Sessions — LanCartographer"), selected: map[int]bool{}}
	v.win.Resize(fyne.NewSize(1040, 760))
	v.summary = widget.NewLabel("")
	v.summary.Wrapping = fyne.TextWrapWord
	v.outcome = widget.NewLabel("")
	v.outcome.Wrapping = fyne.TextWrapWord
	v.context = widget.NewEntry()
	v.context.SetPlaceHolder("Required: site / project / network context")
	v.label = widget.NewEntry()
	v.label.SetPlaceHolder("Optional session label")
	v.notes = widget.NewEntry()
	v.notes.SetPlaceHolder("Optional report notes — included as entered; review for identifiers")
	v.preview = widget.NewLabel("")
	v.preview.Wrapping = fyne.TextWrapWord
	v.previewScroll = container.NewVScroll(v.preview)
	v.redact = widget.NewCheck("Redact identifiers and discovered text", func(bool) { v.updatePreview() })
	v.redact.Checked = true
	v.exportBtn = widget.NewButton("Export selected evidence…", v.export)
	v.saveBtn = widget.NewButton("Save after session…", v.save)
	v.list = widget.NewList(func() int { return len(v.comparison.Changes) }, func() fyne.CanvasObject {
		label := widget.NewLabel("")
		label.Truncation = fyne.TextTruncateEllipsis
		return container.NewBorder(nil, nil, widget.NewCheck("", nil), nil, label)
	}, func(i widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fyne.Container)
		label := row.Objects[0].(*widget.Label)
		check := row.Objects[1].(*widget.Check)
		d := v.comparison.Changes[i]
		label.SetText(fmt.Sprintf("%s · %s: %s -> %s", d.IP, d.Field, session.Value(d.Before), session.Value(d.After)))
		check.OnChanged = nil
		check.SetChecked(v.selected[i])
		check.OnChanged = func(yes bool) { v.selected[i] = yes; v.updatePreview() }
	})
	v.context.OnChanged = func(text string) {
		if v.after != nil {
			v.after.Context = strings.TrimSpace(text)
		}
		v.recompare()
	}
	v.label.OnChanged = func(text string) {
		if v.after != nil {
			v.after.Label = strings.TrimSpace(text)
		}
		v.updateSummary()
		v.updatePreview()
	}
	v.notes.OnChanged = func(string) { v.updatePreview() }
	buttons := container.NewHBox(
		widget.NewButton("Open before…", func() { v.open(true) }),
		widget.NewButton("Open after…", func() { v.open(false) }),
		widget.NewButton("Use current scan", v.useCurrent), v.saveBtn)
	metadata := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("After context"), nil, v.context),
		container.NewBorder(nil, nil, widget.NewLabel("After label"), nil, v.label))
	selectAll := widget.NewButton("Select all", func() {
		for i := range v.comparison.Changes {
			v.selected[i] = true
		}
		v.list.Refresh()
		v.updatePreview()
	})
	clear := widget.NewButton("Clear selection", func() {
		v.selected = map[int]bool{}
		v.list.Refresh()
		v.updatePreview()
	})
	help := widget.NewLabel("Compare recorded address observations. Missing data is not proof of an offline device. Saving/opening/comparing sends no probes.")
	help.Wrapping = fyne.TextWrapWord
	changes := container.NewBorder(container.NewVBox(v.outcome, container.NewHBox(selectAll, clear, v.redact)), nil, nil, nil, v.list)
	report := container.NewBorder(container.NewVBox(widget.NewLabel("Selected evidence preview"), v.notes), v.exportBtn, nil, nil, v.previewScroll)
	split := container.NewVSplit(changes, report)
	split.Offset = .43
	v.win.SetContent(container.NewBorder(container.NewVBox(buttons, metadata, v.summary, help), nil, nil, nil, split))
	if !u.scanning && u.currentSession != nil {
		v.setAfter(u.currentSession.Clone())
	} else {
		v.recompare()
	}
	return v
}

func (v *sessionsView) useCurrent() {
	if v.owner.scanning || v.owner.currentSession == nil {
		dialog.ShowInformation("No finished scan", "Finish or stop a scan first. Favorites are not a scan session.", v.win)
		return
	}
	v.setAfter(v.owner.currentSession.Clone())
}

func (v *sessionsView) setAfter(s session.Session) {
	v.after = &s
	v.context.SetText(s.Context)
	v.label.SetText(s.Label)
	v.recompare()
}

func (v *sessionsView) open(before bool) {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, v.win)
			return
		}
		if r == nil {
			return
		}
		defer r.Close()
		s, err := session.Read(r)
		if err != nil {
			dialog.ShowError(err, v.win)
			return
		}
		if before {
			v.before = &s
			v.recompare()
		} else {
			v.setAfter(s)
		}
	}, v.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	if path, err := appdata.Path("sessions"); err == nil {
		if location, err := storage.ListerForURI(storage.NewFileURI(path)); err == nil {
			fd.SetLocation(location)
		}
	}
	fd.Show()
}

func (v *sessionsView) updateSummary() {
	describe := func(name string, s *session.Session) string {
		if s == nil {
			return name + ": not loaded"
		}
		return fmt.Sprintf("%s: %s · %s · %s · %.0f%% · %d addresses · %s\nContext: %s · %s/%s · %d source addresses recorded (OS routing)",
			name, session.Display(s.Label), s.Target, s.State, s.Progress*100, len(s.Hosts), s.StartedAt.Format(time.RFC3339),
			session.Display(s.Context), session.Display(s.Source.OS), session.Display(s.Source.Arch), len(s.Source.Interfaces))
	}
	v.summary.SetText(describe("Before", v.before) + "\n" + describe("After", v.after))
	if v.after == nil {
		v.saveBtn.Disable()
	} else {
		v.saveBtn.Enable()
	}
}

func (v *sessionsView) recompare() {
	v.selected = map[int]bool{}
	v.comparison = session.Comparison{}
	v.updateSummary()
	if v.before == nil || v.after == nil {
		v.outcome.SetText("Open a before session and an after session, or use the current scan.")
	} else {
		v.comparison = session.Compare(*v.before, *v.after)
		if len(v.comparison.Reasons) > 0 {
			v.outcome.SetText("Not comparable: " + strings.Join(v.comparison.Reasons, " "))
		} else {
			v.outcome.SetText(fmt.Sprintf("%d differences · matching scope/profile/source · both scans complete", len(v.comparison.Changes)))
		}
	}
	v.list.Refresh()
	v.updatePreview()
}

func (v *sessionsView) updatePreview() {
	v.previewScroll.ScrollToTop()
	v.exportBtn.Disable()
	if v.before == nil || v.after == nil {
		v.preview.SetText("Select differences to preview an export.")
		return
	}
	var selected []int
	for i, yes := range v.selected {
		if yes {
			selected = append(selected, i)
		}
	}
	report, err := session.Report(*v.before, *v.after, selected, v.redact.Checked, v.notes.Text)
	if err != nil {
		v.preview.SetText(err.Error())
		return
	}
	v.preview.SetText(report)
	v.exportBtn.Enable()
}

func (v *sessionsView) save() {
	if v.after == nil {
		return
	}
	s := v.after.Clone()
	if err := s.Validate(); err != nil {
		dialog.ShowError(err, v.win)
		return
	}
	name := "session-" + s.StartedAt.UTC().Format("20060102-150405") + ".json"
	v.savePath("Save after session", name, func(path string) error { return session.Save(path, s) })
}

func (v *sessionsView) export() {
	if v.exportBtn.Disabled() {
		return
	}
	data := []byte(v.preview.Text) // freeze the exact preview before the save dialog
	v.savePath("Export selected evidence", "comparison-evidence.txt", func(path string) error { return session.WriteFile(path, data) })
}

// FileSave opens/truncates a destination before its callback. A path form keeps
// replacement atomic and leaves an existing file untouched if encoding fails.
func (v *sessionsView) savePath(title, name string, write func(string) error) {
	path, err := appdata.Path(filepath.Join("sessions", name))
	if err != nil {
		dialog.ShowError(err, v.win)
		return
	}
	entry := widget.NewEntry()
	entry.SetText(path)
	entry.Validator = func(s string) error {
		if !filepath.IsAbs(strings.TrimSpace(s)) {
			return errors.New("enter an absolute file path")
		}
		return nil
	}
	save := func(path string) {
		if err := write(path); err != nil {
			dialog.ShowError(err, v.win)
			return
		}
		dialog.ShowInformation("Saved", path, v.win)
	}
	form := dialog.NewForm(title, "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("File path", entry)}, func(ok bool) {
		if !ok {
			return
		}
		path := strings.TrimSpace(entry.Text)
		if _, err := os.Lstat(path); err == nil {
			dialog.ShowConfirm("Replace existing file?", path, func(yes bool) {
				if yes {
					save(path)
				}
			}, v.win)
		} else if !os.IsNotExist(err) {
			dialog.ShowError(err, v.win)
		} else {
			save(path)
		}
	}, v.win)
	form.Resize(fyne.NewSize(850, 180))
	form.Show()
}

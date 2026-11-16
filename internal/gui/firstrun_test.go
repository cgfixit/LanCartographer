package gui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// buttonNamed returns the first button with the given text under obj.
func buttonNamed(obj fyne.CanvasObject, want string) *widget.Button {
	if b, ok := obj.(*widget.Button); ok && b.Text == want {
		return b
	}
	for _, c := range children(obj) {
		if b := buttonNamed(c, want); b != nil {
			return b
		}
	}
	return nil
}

func TestFirstRunMarkerWrittenOrErrorShown(t *testing.T) {
	t.Run("writable", func(t *testing.T) {
		u := newTestUI(t)
		dir := os.Getenv("APPDATA")
		u.maybeFirstRun()
		test.Tap(buttonNamed(u.win.Canvas().Overlays().Top(), "I understand"))
		if top := u.win.Canvas().Overlays().Top(); top != nil {
			t.Fatal("a dialog is still open after a successful acknowledgement")
		}
		if _, err := os.Stat(filepath.Join(dir, "LanCartographer", "firstrun_seen")); err != nil {
			t.Fatalf("marker not written: %v", err)
		}
	})
	t.Run("unwritable", func(t *testing.T) {
		u := newTestUI(t)
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("APPDATA", filepath.Join(blocker, "sub")) // MkdirAll fails: parent is a file
		u.maybeFirstRun()
		test.Tap(buttonNamed(u.win.Canvas().Overlays().Top(), "I understand"))
		top := u.win.Canvas().Overlays().Top()
		if top == nil || buttonNamed(top, "I understand") != nil || buttonNamed(top, "OK") == nil {
			t.Fatal("expected an error dialog explaining the marker could not be saved")
		}
	})
}

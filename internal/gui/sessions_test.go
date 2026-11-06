//go:build cgo

package gui

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
	"github.com/local/lancartographer/internal/session"
)

func TestSessionCaptureCompletionCancellationAndFavorites(t *testing.T) {
	u := newTestUI(t)
	start := func() {
		s := session.New(netip.MustParsePrefix("192.0.2.0/24"), probe.Options{Ports: []int{80}, DialTimeout: 400 * time.Millisecond, ProbeTimeout: 800 * time.Millisecond, Concurrency: 128}, Version)
		u.currentSession = &s
		u.scanDone = false
		u.store = newHostStore()
	}
	start()
	u.applyEvent(portEvent("192.0.2.10", 80))
	u.applyEvent(scan.Event{Kind: scan.EventProgress, Progress: .5})
	u.endSession(context.Canceled)
	if u.currentSession.State != session.Cancelled || u.currentSession.Progress != .5 || len(u.currentSession.Hosts) != 1 {
		t.Fatal("cancelled snapshot lost partial evidence")
	}
	start()
	u.applyEvent(scan.Event{Kind: scan.EventHostFound, Host: model.Host{IP: netip.MustParseAddr("192.0.2.10"), Name: "baseline", Ports: []model.Port{{Number: 80, State: model.PortOpen}}}})
	u.applyEvent(scan.Event{Kind: scan.EventDone, Progress: 1})
	u.endSession(nil)
	if u.currentSession.State != session.Complete || u.currentSession.FinishedAt.IsZero() {
		t.Fatal("completed snapshot missing terminal metadata")
	}
	u.store.hosts["192.0.2.10"].Ports[0].State = model.PortClosed
	if u.currentSession.Hosts[0].Ports[0].State != model.PortOpen {
		t.Fatal("snapshot changed with live inventory")
	}
	u.onSaveFavorites()
	u.onLoadFavorites()
	if u.currentSession != nil {
		t.Fatal("favorites retained scan coverage")
	}
}

func TestSessionsSelectionPreviewAndSnapshotOwnership(t *testing.T) {
	u := newTestUI(t)
	s := session.New(netip.MustParsePrefix("192.0.2.0/24"), probe.Options{Ports: []int{80}, DialTimeout: 400 * time.Millisecond, ProbeTimeout: 800 * time.Millisecond, Concurrency: 128}, Version)
	s.Context, s.Label, s.State, s.Progress = "private-site", "After", session.Complete, 1
	s.FinishedAt = s.StartedAt.Add(time.Second)
	s.Source = session.Source{OS: "darwin", Arch: "arm64", Interfaces: []string{"fixture 192.0.2.1/24"}}
	s.Hosts = []model.Host{{IP: netip.MustParseAddr("192.0.2.10"), Name: "private-after", Ports: []model.Port{{Number: 80, State: model.PortOpen}}}}
	u.currentSession = &s
	v := u.newSessionsView()
	defer v.win.Close()
	before := s.Clone()
	before.Hosts[0].Name = "private-before"
	v.before = &before
	v.recompare()
	if len(v.comparison.Changes) != 1 || !v.exportBtn.Disabled() {
		t.Fatal("unexpected initial comparison/selection")
	}
	row := v.list.CreateItem().(*fyne.Container)
	v.list.UpdateItem(0, row)
	test.Tap(row.Objects[1].(*widget.Check))
	if v.exportBtn.Disabled() || !strings.Contains(v.preview.Text, "Selected differences: 1 of 1") || strings.Contains(v.preview.Text, "private") {
		t.Fatal("selection did not produce a redacted preview")
	}
	test.Tap(v.redact)
	if !strings.Contains(v.preview.Text, "private-after") {
		t.Fatal("explicit identifier opt-in did not update preview")
	}
	v.previewScroll.Offset.Y = 200
	v.context.SetText("different-site")
	if v.previewScroll.Offset.Y != 0 || u.currentSession.Context != "private-site" || !v.exportBtn.Disabled() || !strings.Contains(v.outcome.Text, "Not comparable") {
		t.Fatal("editing context changed live state or retained stale evidence")
	}
	v.setAfter(s.Clone())
	v.after.State = session.Cancelled
	v.recompare()
	if len(v.comparison.Changes) != 0 || !strings.Contains(v.outcome.Text, "incomplete") {
		t.Fatal("partial scan compared as completed")
	}
}

func TestSessionsOpenFileDialog(t *testing.T) {
	u := newTestUI(t)
	v := u.newSessionsView()
	defer v.win.Close()
	v.win.Show()
	v.open(true)
	if len(v.win.Canvas().Overlays().List()) == 0 {
		t.Fatal("opening a session did not show a file dialog")
	}
}

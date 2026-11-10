//go:build cgo

package gui

import (
	"net/netip"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/scan"
)

func portEvent(ip string, port int) scan.Event {
	return scan.Event{
		Kind: scan.EventPortFound,
		Host: model.Host{
			IP:    netip.MustParseAddr(ip),
			Ports: []model.Port{{Number: port, State: model.PortOpen}},
		},
	}
}

// TestApplyEventCoalescesRapidRepaints proves the scan hot path applies
// every event to the store but does not pay a full sort+repaint per event:
// two rapid PortFound events inside the throttle window must coalesce into
// zero repaints, and finishScan must flush the exact settled state. The
// throttle is widened to a minute so the test never depends on wall time.
func TestApplyEventCoalescesRapidRepaints(t *testing.T) {
	u := newTestUI(t)
	u.throttle = time.Minute
	base := u.repaints

	u.applyEvent(portEvent("192.168.1.10", 445))
	u.applyEvent(portEvent("192.168.1.11", 22))

	if got := u.repaints - base; got != 0 {
		t.Fatalf("two rapid events caused %d full refreshes, want 0 (throttled)", got)
	}
	if got := u.store.len(); got != 2 {
		t.Fatalf("store.len() = %d, want 2: store updates must apply per event", got)
	}

	u.finishScan()

	if got := u.repaints - base; got != 1 {
		t.Fatalf("finishScan triggered %d flushes, want 1", got)
	}
	if got := len(u.sorted); got != 2 {
		t.Fatalf("len(sorted) = %d after flush, want 2", got)
	}
}

// TestApplyEventDoneAlwaysFlushes proves the terminal event bypasses the
// throttle even when the previous event just repainted.
func TestApplyEventDoneAlwaysFlushes(t *testing.T) {
	u := newTestUI(t)
	u.throttle = time.Minute
	u.refreshTable() // fresh lastRefresh inside the throttle window
	base := u.repaints

	u.applyEvent(scan.Event{Kind: scan.EventDone, Progress: 1})

	if got := u.repaints - base; got != 1 {
		t.Fatalf("EventDone caused %d full refreshes, want 1 (never throttled)", got)
	}
}

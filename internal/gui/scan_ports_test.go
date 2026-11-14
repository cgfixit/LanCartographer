//go:build cgo

package gui

import (
	"context"
	"strings"
	"testing"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
)

func TestScanPortControls(t *testing.T) {
	u := newTestUI(t)
	if u.ports.Text != probe.DefaultPortsCSV {
		t.Fatalf("initial ports = %q", u.ports.Text)
	}
	u.ports.SetText("3000,5432")
	u.resetPorts.OnTapped()
	if u.ports.Text != probe.DefaultPortsCSV {
		t.Fatalf("restored ports = %q", u.ports.Text)
	}
}

func TestInvalidScanPortsPreserveInventory(t *testing.T) {
	u := newTestUI(t)
	h := host("192.168.1.5", model.Port{Number: 3000, State: model.PortOpen})
	u.store.set([]model.Host{h})
	u.refreshTable()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	u.qualityJob = &qualityJob{ctx: ctx, cancel: cancel}
	for _, invalid := range []string{"", "0", "65536", "3000,", "3000-3010", "http"} {
		u.ports.SetText(invalid)
		u.onScan()
		if !strings.Contains(u.statusErr.Text, "TCP ports:") {
			t.Fatalf("%q: missing port error: %q", invalid, u.statusErr.Text)
		}
		if u.scanning || ctx.Err() != nil || u.qualityJob.scanAfter.IsValid() || len(u.sorted) != 1 || u.sorted[0].IP != h.IP {
			t.Fatalf("%q: invalid input started work or replaced inventory", invalid)
		}
	}
}

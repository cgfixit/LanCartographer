//go:build cgo

package gui

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/local/lancartographer/internal/discover"
)

// The pending measurement holds any accepted scan in the queue so a gate
// regression fails an assertion without starting an engine or contacting a host.
func holdScan(t *testing.T, u *ui) *qualityJob {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	job := &qualityJob{view: &qualityView{}, ctx: ctx, cancel: cancel}
	u.qualityJob = job
	return job
}

func TestScanUsesCurrentSubnets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target string
		err    error
		prompt bool
	}{
		{"previous network", "192.168.1.0/24", nil, true},
		{"current network", "10.11.12.0/24", nil, false},
		{"discovery failure", "192.168.1.0/24", errors.New("interface unavailable"), true},
		{"public range", "192.0.2.5/32", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI(t)
			job := holdScan(t, u)
			u.deps.localSubnets = func(virtual bool) ([]discover.InterfaceInfo, error) {
				if virtual {
					t.Fatal("virtual interfaces included in the gate")
				}
				if tc.err != nil {
					return []discover.InterfaceInfo{{Prefix: netip.MustParsePrefix("192.168.1.0/24")}}, tc.err
				}
				return []discover.InterfaceInfo{{Prefix: netip.MustParsePrefix("10.11.12.1/24")}}, nil
			}
			u.subnet.SetText(tc.target)
			u.onScan()
			top := u.win.Canvas().Overlays().Top()
			if (top != nil) != tc.prompt || u.subnet.Text != tc.target {
				t.Fatalf("prompt = %v, target = %q", top != nil, u.subnet.Text)
			}
			if tc.prompt {
				no := buttonNamed(top, "No")
				if no == nil {
					t.Fatal("missing refusal button")
				}
				test.Tap(no)
				if job.ctx.Err() != nil || job.scanAfter.IsValid() || u.currentSession != nil {
					t.Fatal("declined scan changed the measurement, queue or inventory")
				}
			} else if job.scanAfter.String() != tc.target {
				t.Fatalf("queued target = %v", job.scanAfter)
			}
		})
	}
}

func TestServiceConfirmationRefreshesScope(t *testing.T) {
	u := newTestUI(t)
	u.showServiceCheck(&servicePrefill{host: "192.168.1.20", port: 443})
	v := u.serviceView
	t.Cleanup(v.win.Close)
	u.deps.localSubnets = func(bool) ([]discover.InterfaceInfo, error) { return nil, nil }
	target, err := v.serviceTarget()
	if err != nil {
		t.Fatal(err)
	}
	u.confirmServiceCheck(v)
	top := v.win.Canvas().Overlays().Top()
	if top == nil || !findLabel(top, serviceConfirmText(target, nil)) {
		t.Fatal("the confirmation should warn that the selected address is outside the current subnets")
	}
	test.Tap(buttonNamed(top, "No"))
	if v.confirming || v.job != nil || v.last != nil {
		t.Fatal("declined check left a prompt, job or result")
	}
}

func TestQueuedScanChecksScopeBeforeDispatch(t *testing.T) {
	u := newTestUI(t)
	job := holdScan(t, u)
	job.scanAfter = netip.MustParsePrefix("127.0.0.1/32")
	job.portsAfter = []int{8080}
	u.scanBtn.Disable()
	u.ports.Disable()
	u.resetPorts.Disable()
	u.finishInternetQuality(job, context.Canceled, nil)
	top := u.win.Canvas().Overlays().Top()
	if top == nil || u.scanning {
		t.Fatal("resumed scan requires confirmation before starting")
	}
	test.Tap(buttonNamed(top, "No"))
	if u.scanBtn.Disabled() || u.ports.Disabled() || u.resetPorts.Disabled() || u.currentSession != nil {
		t.Fatal("refusing the queued scan should leave inventory idle and controls available")
	}
}

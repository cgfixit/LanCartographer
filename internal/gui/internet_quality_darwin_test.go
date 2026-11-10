//go:build cgo

package gui

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestInternetQualityResults(t *testing.T) {
	valid := `{"dl_throughput":23614442,"ul_throughput":90155512,"base_rtt":26.75,"responsiveness":76.85,"interface_name":"en0"}`
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{"complete", valid, ""},
		{"legacy idle field", strings.Replace(valid, "base_rtt", "base_rt", 1), ""},
		{"malformed", "{", "unreadable"},
		{"trailing data", valid + "{}", "unreadable"},
		{"exit-zero provider error", `{"error_domain":"NSURLErrorDomain","error_code":-1009}`, "could not complete"},
		{"incomplete", `{"dl_throughput":0}`, "incomplete"},
		{"invalid metric", strings.Replace(valid, "23614442", "1e400", 1), "unreadable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := runQualityCommand(t.Context(), "/usr/bin/printf", "%s", tc.data)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want %q", err, tc.want)
				}
				return
			}
			if err != nil || *r.Download != 23614442 || *r.Upload != 90155512 || *r.Idle != 26.75 || *r.RPM != 76.85 {
				t.Fatalf("result = %+v, error = %v", r, err)
			}
		})
	}
}

func TestInternetQualityProcessBounds(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := runQualityCommand(ctx, "/bin/sleep", "10"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("command was not reaped promptly")
	}
	ctx, cancel = context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err := runQualityCommand(ctx, "/usr/bin/yes"); !errors.Is(err, errQualityOutputLimit) {
		t.Fatalf("output cap error = %v", err)
	}
	if _, err := runQualityCommand(t.Context(), t.TempDir()+"/missing"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing command error = %v", err)
	}
}

func TestInternetQualityScanWaitsForMeasurement(t *testing.T) {
	u := newTestUI(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	job := &qualityJob{view: &qualityView{}, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	u.qualityJob = job
	target := netip.MustParsePrefix("127.0.0.1/32")
	u.startScan(target)
	if ctx.Err() != context.Canceled || job.scanAfter != target {
		t.Fatalf("scan did not cancel the measurement and queue its approved target: %v, %v", ctx.Err(), job.scanAfter)
	}
	u.startScan(netip.MustParsePrefix("127.0.0.2/32"))
	if u.scanning || u.currentSession != nil || job.scanAfter != target {
		t.Fatalf("scan changed inventory or replaced its queued target before reap: %v, %v", u.scanning, job.scanAfter)
	}
}

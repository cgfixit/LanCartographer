package scan

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"golang.org/x/sys/unix"
)

func TestRun_ICMPOnly(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	opts := probe.Options{Ports: []int{port}, DialTimeout: 50 * time.Millisecond, ProbeTimeout: 100 * time.Millisecond, Concurrency: 2}
	d := defaultDeps(opts)
	// Simulate filtered TCP and an empty ARP cache; ICMP uses the real OS probe.
	d.dial = func(context.Context, netip.Addr, int) bool { return false }
	d.arp = func(netip.Addr) bool { return false }
	events, err := runCollect(context.Background(), netip.MustParsePrefix("127.0.0.1/32"), opts, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Kind != EventHostFound {
			continue
		}
		host := event.Host
		if !host.Alive || host.FoundVia != "icmp" || len(host.Ports) != 1 || host.Ports[0].State != model.PortClosed {
			t.Fatalf("expected an ICMP-discovered host with a closed TCP port, got %+v", host)
		}
		return
	}
	t.Fatal("ICMP-responsive host with no open TCP ports was omitted")
}

func TestRun_StopReapsSMBUtil(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "pid")
	script := "#!/bin/sh\nprintf '%s\\n' \"$$\" > \"$LAN_TEST_SMB_PID\"\nexec /bin/sleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "smbutil"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("LAN_TEST_SMB_PID", marker)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	opts := probe.Options{Ports: []int{445}}
	deps := fakeAliveDeps(func(emit func(model.Port)) {
		emit(model.Port{Number: 445, State: model.PortOpen, Service: "smb"})
	}, nil, nil, "", "", "", nil, true)
	deps.listShares = defaultDeps(opts).listShares
	type result struct {
		events []Event
		err    error
	}
	done := make(chan result, 1)
	go func() {
		events, err := runCollect(ctx, netip.MustParsePrefix("127.0.0.1/32"), opts, deps)
		done <- result{events, err}
	}()

	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(marker)
		if err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid <= 0 {
		t.Fatal("owned smbutil fixture did not start")
	}
	cancel()
	select {
	case r := <-done:
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("scan returned %v, want context cancellation", r.err)
		}
		if countKind(r.events, EventShareFound) != 0 || countKind(r.events, EventDone) != 0 {
			t.Fatalf("cancelled scan emitted share or completion events: %v", r.events)
		}
	case <-time.After(time.Second):
		t.Fatal("enumeration did not stop within one second")
	}
	if err := unix.Kill(pid, 0); !errors.Is(err, unix.ESRCH) {
		t.Errorf("smbutil PID %d was not reaped when the scan stopped: %v", pid, err)
		// Let the baseline subprocess reach its original timeout on failure.
		for deadline := time.Now().Add(6 * time.Second); time.Now().Before(deadline) && unix.Kill(pid, 0) == nil; {
			time.Sleep(20 * time.Millisecond)
		}
	}
}

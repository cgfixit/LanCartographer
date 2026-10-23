package scan

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
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

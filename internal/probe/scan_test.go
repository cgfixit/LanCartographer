package probe

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/model"
)

// httpFixture starts a listener on 127.0.0.1 that answers every connection
// with a minimal HTTP/1.0 response, and returns its port.
func httpFixture(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:18080")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 512)
				c.SetDeadline(time.Now().Add(2 * time.Second))
				if _, err := c.Read(buf); err != nil {
					return
				}
				fmt.Fprintf(c, "HTTP/1.0 200 OK\r\nContent-Length: 0\r\n\r\n")
			}(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// closedPort returns a loopback port that is guaranteed closed: we bind it,
// learn the port, then release it before returning.
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return port
}

func collect(t *testing.T, ctx context.Context, ip netip.Addr, opts Options) []model.Port {
	t.Helper()
	var mu sync.Mutex
	var got []model.Port
	ScanHost(ctx, ip, opts, NewLimiter(opts.Concurrency), func(p model.Port) {
		mu.Lock()
		got = append(got, p)
		mu.Unlock()
	})
	return got
}

func TestScanHost_OpenWebPort(t *testing.T) {
	port := httpFixture(t)
	opts := Options{
		Ports:        []int{port},
		DialTimeout:  400 * time.Millisecond,
		ProbeTimeout: 800 * time.Millisecond,
		Concurrency:  8,
	}
	got := collect(t, context.Background(), netip.MustParseAddr("127.0.0.1"), opts)
	if len(got) != 1 {
		t.Fatalf("emitted %d ports, want 1", len(got))
	}
	want := model.Port{Number: 18080, State: model.PortOpen, Service: "Web", Title: ""}
	if got[0] != want {
		t.Fatalf("got %+v, want %+v", got[0], want)
	}
}

func TestScanHost_ClosedPort(t *testing.T) {
	port := closedPort(t)
	opts := Options{
		Ports:        []int{port},
		DialTimeout:  400 * time.Millisecond,
		ProbeTimeout: 800 * time.Millisecond,
		Concurrency:  8,
	}
	got := collect(t, context.Background(), netip.MustParseAddr("127.0.0.1"), opts)
	if len(got) != 1 {
		t.Fatalf("emitted %d ports, want 1", len(got))
	}
	if got[0].Number != port || got[0].State != model.PortClosed {
		t.Fatalf("got %+v, want {%d closed ...}", got[0], port)
	}
}

func TestScanPort_ExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	got := scanPort(ctx, netip.MustParseAddr("127.0.0.1"), 8080, Options{ProbeTimeout: time.Second})
	if got.State != model.PortTimeout {
		t.Fatalf("got %+v, want state timeout", got)
	}
}

func TestDialFailureState(t *testing.T) {
	for _, err := range []error{syscall.EMFILE, syscall.ENETUNREACH} {
		dialErr := &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: err}}
		if got := dialFailureState(dialErr); got != model.PortUnconfirmed {
			t.Errorf("%v = %q, want unconfirmed", dialErr, got)
		}
	}
	if got := dialFailureState(context.Canceled); got != model.PortTimeout {
		t.Errorf("canceled dial = %q, want timeout", got)
	}
}

func TestClassifyPort(t *testing.T) {
	cases := map[int]string{
		80: "Web", 443: "Web", 8080: "Web", 8443: "Web",
		445: "SMB", 3389: "RDP", 22: "SSH", 21: "FTP",
		515: "Printer", 631: "Printer", 9100: "Printer",
		1: "unknown", 18080: "unknown", 65535: "unknown",
	}
	for port, want := range cases {
		if got := classifyPort(port); got != want {
			t.Errorf("classifyPort(%d) = %q, want %q", port, got, want)
		}
	}
}

package probe

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/local/lancartographer/internal/model"
)

// Options controls a single host's TCP connect port scan.
type Options struct {
	// Ports is the list of TCP ports to probe.
	Ports []int
	// DialTimeout is the per-host dial timeout (400ms discovery default,
	// set by engine/CLI callers).
	DialTimeout time.Duration
	// ProbeTimeout is the per-dial timeout for the connect scan (800ms
	// port-probe default, set by engine/CLI callers). A dial that exceeds
	// it is reported as timeout.
	ProbeTimeout time.Duration
	// Concurrency is the dial concurrency requested by the caller; it is
	// bounded by the Limiter passed to ScanHost.
	Concurrency int
}

// ScanHost probes every port in opts.Ports on ip with a TCP connect scan
// (net.Dialer.DialContext only — no raw sockets, no SYN), bounded by lim,
// and calls emit once per probed port. emit is serialized by an internal
// mutex, so callers need not make it concurrency-safe; ports may be
// emitted in any order.
//
// Cancellation policy (skip-with-ctx-check): a port whose ctx is already
// cancelled before its dial starts is skipped silently (not emitted); a
// dial cancelled in flight reports whatever the dialer observed — a
// deadline/cancellation surfaces as model.PortTimeout, a refusal as
// model.PortClosed.
func ScanHost(ctx context.Context, ip netip.Addr, opts Options, lim *Limiter, emit func(model.Port)) {
	var wg sync.WaitGroup
	var emitMu sync.Mutex
	for _, port := range opts.Ports {
		// Skip-with-ctx-check: do not start new dials once the caller's
		// context is done; ports never started are simply not emitted.
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			if err := lim.Do(ctx, func() {
				p := scanPort(ctx, ip, port, opts)
				emitMu.Lock()
				emit(p)
				emitMu.Unlock()
			}); err != nil {
				// Lost the slot race to ctx cancellation — skip this port.
				return
			}
		}(port)
	}
	wg.Wait()
}

// scanPort performs one connect probe and classifies the result.
func scanPort(ctx context.Context, ip netip.Addr, port int, opts Options) model.Port {
	d := net.Dialer{Timeout: opts.ProbeTimeout}
	conn, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(ip, uint16(port)).String())
	if err != nil {
		var nerr net.Error
		if errors.As(err, &nerr) && nerr.Timeout() {
			return model.Port{Number: port, State: model.PortTimeout}
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return model.Port{Number: port, State: model.PortTimeout}
		}
		return model.Port{Number: port, State: model.PortClosed}
	}
	conn.Close()
	p := model.Port{Number: port, State: model.PortOpen, Service: classifyPort(port)}
	if p.Service == "unknown" {
		// R7 seam: promote open unknown ports that answer an HTTP probe.
		dctx, cancel := context.WithTimeout(ctx, time.Second)
		title, ok := detectWeb(dctx, ip, port)
		cancel()
		if ok {
			p.Service = "Web"
			p.Title = title
		}
	}
	return p
}

// classifyPort maps a TCP port number to its pinned service label.
func classifyPort(port int) string {
	switch port {
	case 80, 443, 8080, 8443:
		return "Web"
	case 445:
		return "SMB"
	case 3389:
		return "RDP"
	case 22:
		return "SSH"
	case 21:
		return "FTP"
	case 515, 631, 9100:
		return "Printer"
	default:
		return "unknown"
	}
}

// detectWeb is the unexported seam behind web-service promotion (ruling
// R7). Minimal version: dial with a 1s timeout, send a bare HTTP/1.0 GET,
// read at most 2 KiB, and report ok only when the first line parses as
// "HTTP/1.x <3-digit status>". Title extraction arrives in Task 8, so the
// returned title is always empty for now.
func detectWeb(ctx context.Context, ip netip.Addr, port int) (title string, ok bool) {
	d := net.Dialer{Timeout: time.Second}
	conn, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(ip, uint16(port)).String())
	if err != nil {
		return "", false
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(time.Second))
	}
	if _, err := conn.Write([]byte("GET / HTTP/1.0\r\n\r\n")); err != nil {
		return "", false
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if n == 0 {
		return "", false
	}
	_ = err // a partial read still counts; only the bytes matter
	line := string(buf[:n])
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	line = strings.TrimRight(line, "\r")
	fields := strings.SplitN(line, " ", 3)
	if len(fields) < 2 {
		return "", false
	}
	proto, status := fields[0], fields[1]
	if len(proto) != len("HTTP/1.x") || !strings.HasPrefix(proto, "HTTP/1.") ||
		proto[len(proto)-1] < '0' || proto[len(proto)-1] > '9' {
		return "", false
	}
	if len(status) != 3 {
		return "", false
	}
	for _, c := range status {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return "", true
}

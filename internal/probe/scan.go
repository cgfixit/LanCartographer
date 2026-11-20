package probe

import (
	"context"
	"errors"
	"net"
	"net/netip"
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
	if p.Service == "Web" {
		// Known web ports need the same bounded title enrichment as an
		// unknown port promoted to Web. Keep certificate checks enabled.
		var cert *leafCert
		_, p.Title, cert, _ = httpGet(ctx, ip, port, port == 443 || port == 8443)
		if cert != nil {
			p.CertExpires, p.CertTrusted = cert.NotAfter.UTC().Format(time.RFC3339), cert.Trusted
		}
	}
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

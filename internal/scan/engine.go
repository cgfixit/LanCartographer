// Package scan is the central sweep engine: it enumerates a target prefix,
// fans hosts out through a worker pool, and streams what it finds as Events
// on a channel. Live port/share events arrive during probing; exactly one
// canonical EventHostFound arrives per alive host after enrichment. Per-host
// failures degrade to empty fields — never to error events.
package scan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/local/lancartographer/internal/discover"
	"github.com/local/lancartographer/internal/hostcache"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/oui"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/winutil"
)

// EventKind identifies what an Event carries.
type EventKind int

const (
	// EventHostFound is the single canonical, fully enriched host record,
	// emitted once per alive host after probing and enrichment complete.
	EventHostFound EventKind = iota
	// EventPortFound is a live observation of one open port during probing.
	EventPortFound
	// EventShareFound is a live observation of one SMB share during probing
	// (including the synthetic access-denied share).
	EventShareFound
	// EventProgress reports sweep progress as completed/total after each
	// host finishes, alive or dead.
	EventProgress
	// EventDone is the final event (Progress=1) on full completion.
	EventDone
	// EventError is reserved for engine-level failures; per-host failures
	// never produce it.
	EventError
)

// Event is one unit of scan output on the Run channel.
type Event struct {
	Kind     EventKind
	Host     model.Host
	Progress float64
	Err      error
}

// maxHostsInFlight bounds concurrent per-host processing (R8: a /24 must
// finish in seconds, which sequential liveness cannot do).
const maxHostsInFlight = 64

// maxPrefixHostBits bounds the address space a sweep may span: 2^20
// addresses (a /12). Anything broader would materialize a host list large
// enough to hang or OOM the operator's own machine, so Run rejects it
// before enumeration.
const maxPrefixHostBits = 20

// validateTarget rejects targets Run cannot safely sweep: non-IPv4 ranges
// (IPv6 enumeration is unbounded and unsupported) and IPv4 prefixes broader
// than a /12. An invalid (zero) prefix is allowed: it sweeps zero hosts,
// preserving the empty-scan no-op.
func validateTarget(target netip.Prefix) error {
	if !target.IsValid() {
		return nil
	}
	if !target.Addr().Is4() {
		return fmt.Errorf("scan: only IPv4 ranges are supported, got %s", target)
	}
	if hostBits := 32 - target.Bits(); hostBits > maxPrefixHostBits {
		return fmt.Errorf("scan: range too large: %s spans %d addresses; maximum is /12 (%d addresses)",
			target.Masked(), uint64(1)<<hostBits, uint64(1)<<maxPrefixHostBits)
	}
	return nil
}

// deps is the testability seam (R3): every external effect the engine needs
// binds here, with defaultDeps wiring the real implementations.
type deps struct {
	dial       discover.DialFunc                                                                  // one TCP connect attempt
	arp        func(netip.Addr) bool                                                              // ARP-table fallback for liveness
	scan       func(context.Context, netip.Addr, probe.Options, *probe.Limiter, func(model.Port)) // port scan
	listShares func(context.Context, netip.Addr) ([]model.Share, error)                           // SMB share enumeration
	nbName     func(context.Context, netip.Addr) string                                           // NetBIOS name
	mdnsName   func(context.Context, netip.Addr) string                                           // mDNS PTR name
	revName    func(context.Context, netip.Addr) string                                           // reverse-DNS fallback name
	lookupMAC  func(netip.Addr) (string, error)                                                   // SendARP MAC resolution
	isElevated func() bool                                                                        // process token elevation
	oui        func(string) string                                                                // MAC vendor lookup
	loadNames  func() map[string]hostcache.Entry                                                  // MAC-keyed hostname cache load
	saveNames  func(map[string]hostcache.Entry)                                                   // hostname cache persist
}

// defaultDeps binds the real package functions. opts supplies the liveness
// dial timeout; the other bindings are fixed.
func defaultDeps(opts probe.Options) deps {
	return deps{
		dial:       tcpDial(opts.DialTimeout),
		arp:        arpPresent,
		scan:       probe.ScanHost,
		listShares: func(_ context.Context, ip netip.Addr) ([]model.Share, error) { return probe.ListShares(ip) },
		nbName:     probe.NetBIOSName,
		mdnsName:   probe.MDNSName,
		revName:    probe.ReverseName,
		lookupMAC:  probe.LookupMAC,
		isElevated: winutil.IsElevated,
		oui:        oui.Lookup,
		loadNames: func() map[string]hostcache.Entry {
			m, _ := hostcache.Load()
			return m
		},
		saveNames: func(m map[string]hostcache.Entry) { _ = hostcache.Save(m) },
	}
}

// tcpDial is the real DialFunc: one TCP connect, closed immediately.
func tcpDial(timeout time.Duration) discover.DialFunc {
	return func(ctx context.Context, ip netip.Addr, port int) bool {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", netip.AddrPortFrom(ip, uint16(port)).String())
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}
}

// arpPresent reports whether ip has an ARP-table entry.
func arpPresent(ip netip.Addr) bool {
	_, err := winutil.SendARP(ip)
	return err == nil
}

// Run sweeps target and streams events on out. It returns nil on full
// completion (after emitting EventDone with Progress=1), ctx.Err() when
// cancelled, and a validation error (without emitting any event) when
// target is not IPv4 or spans more than a /12. out is never closed by Run;
// no events are sent after Run returns, so the caller may close it
// immediately.
func Run(ctx context.Context, target netip.Prefix, opts probe.Options, out chan<- Event) error {
	return run(ctx, target, opts, out, defaultDeps(opts))
}

func run(ctx context.Context, target netip.Prefix, opts probe.Options, out chan<- Event, d deps) error {
	if err := validateTarget(target); err != nil {
		return err
	}
	hosts := discover.HostsIn(target.Masked())
	total := len(hosts)
	if total == 0 {
		out <- Event{Kind: EventDone, Progress: 1}
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The MAC-keyed hostname cache loads once before any worker starts and
	// persists once on return; workers merge through the mutex-guarded view,
	// so no file I/O lands in the hot path.
	var names map[string]hostcache.Entry
	if d.loadNames != nil {
		names = d.loadNames()
	}
	if names == nil {
		names = map[string]hostcache.Entry{}
	}
	cache := &nameCache{m: names}
	defer func() {
		if d.saveNames != nil {
			d.saveNames(cache.m)
		}
	}()

	em := &emitter{ctx: ctx, out: out, total: total}
	lim := probe.NewLimiter(opts.Concurrency) // single global dial budget (R8)

	workers := maxHostsInFlight
	if total < workers {
		workers = total
	}
	jobs := make(chan netip.Addr)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				if ctx.Err() != nil {
					return
				}
				runHost(ctx, ip, opts, lim, em, d, cache)
				if !em.send(Event{Kind: EventProgress}) {
					return
				}
			}
		}()
	}

feed:
	for _, ip := range hosts {
		select {
		case jobs <- ip:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}
	em.send(Event{Kind: EventDone, Progress: 1})
	return nil
}

// emitter serializes channel sends so concurrent workers cannot interleave
// them, and aborts a blocked send when the sweep context is cancelled so
// Run can unwind promptly.
type emitter struct {
	ctx context.Context
	out chan<- Event
	mu  sync.Mutex
	// Count completion under the send lock: an atomic increment before
	// send lets a later worker publish its larger count first.
	completed int
	total     int
}

func (e *emitter) send(ev Event) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ev.Kind == EventProgress {
		e.completed++
		ev.Progress = float64(e.completed) / float64(e.total)
	}
	select {
	case e.out <- ev:
		return true
	case <-e.ctx.Done():
		return false
	}
}

// runHost probes one host end to end on hctx (a child of the sweep ctx, so
// Stop cancels in-flight dials) and emits its events. Every failure inside
// degrades to an empty field; the host is reported as long as it is alive.
func runHost(ctx context.Context, ip netip.Addr, opts probe.Options, lim *probe.Limiter, em *emitter, d deps, cache *nameCache) {
	hctx, cancel := context.WithCancel(ctx)
	defer cancel()

	alive, foundVia := discover.Alive(hctx, ip, lim, d.dial, d.arp, opts.Ports...)
	if !alive {
		return
	}
	h := model.Host{IP: ip, Alive: true, FoundVia: foundVia}

	var mu sync.Mutex // guards h.Ports and port445 across emit callbacks
	port445 := false
	d.scan(hctx, ip, opts, lim, func(p model.Port) {
		mu.Lock()
		h.Ports = append(h.Ports, p)
		if p.State == model.PortOpen && p.Number == 445 {
			port445 = true
		}
		mu.Unlock()
		if p.State == model.PortOpen {
			em.send(Event{Kind: EventPortFound, Host: model.Host{IP: ip, Ports: []model.Port{p}}})
		}
	})
	sort.Slice(h.Ports, func(i, j int) bool { return h.Ports[i].Number < h.Ports[j].Number })

	// Shares are probed only when the port scan found 445 open.
	if port445 {
		if !probeShares(hctx, &h, em, d) {
			return // cancelled mid-share-probe: abandon the host
		}
	}

	// Enrichment: failure is an empty field, not an error row.
	name := resolveName(hctx, ip, d)
	if mac, err := d.lookupMAC(ip); err == nil {
		h.MAC = mac
		if mac != "" && mac != probe.MacNeedsElevation {
			h.Vendor = d.oui(mac)
			if name != "" {
				cache.put(mac, name)
			} else if cached, ok := cache.get(mac); ok {
				name = cached
			}
		}
	} else if runtime.GOOS == "windows" && !d.isElevated() {
		h.MAC = probe.MacNeedsElevation
	}
	h.Name = name

	em.send(Event{Kind: EventHostFound, Host: h})
}

// resolveName asks all three name services concurrently and picks the first
// non-empty answer by priority: NetBIOS, then mDNS, then reverse DNS. Each
// probe gets its own bounded ctx (NetBIOS/mDNS ~900ms, PTR 1.5s) so the
// slowest of them, not their sum, bounds the wait. On host cancellation the
// pending probes unwind through their ctx and an empty name is returned.
func resolveName(hctx context.Context, ip netip.Addr, d deps) string {
	nbCtx, nbCancel := context.WithTimeout(hctx, 900*time.Millisecond)
	defer nbCancel()
	revCtx, revCancel := context.WithTimeout(hctx, 1500*time.Millisecond)
	defer revCancel()

	type res struct{ nb, mdns, rev string }
	ch := make(chan res, 1) // buffered: an abandoned collection never blocks
	go func() {
		var r res
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); r.nb = d.nbName(nbCtx, ip) }()
		go func() { defer wg.Done(); r.mdns = d.mdnsName(nbCtx, ip) }()
		go func() { defer wg.Done(); r.rev = d.revName(revCtx, ip) }()
		wg.Wait()
		ch <- r
	}()

	select {
	case r := <-ch:
		if r.nb != "" {
			return r.nb
		}
		if r.mdns != "" {
			return r.mdns
		}
		return r.rev
	case <-hctx.Done():
		return ""
	}
}

// nameCache wraps the shared hostname map so concurrent workers can merge
// without racing; the map itself is persisted once by run on return.
type nameCache struct {
	mu sync.Mutex
	m  map[string]hostcache.Entry
}

func (c *nameCache) get(mac string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return hostcache.Map(c.m).Get(mac)
}

func (c *nameCache) put(mac, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]hostcache.Entry{}
	}
	hostcache.Map(c.m).Put(mac, name)
}

// probeShares enumerates the host's shares and emits one live EventShareFound
// per share. Access denied yields a single synthetic denial share (R10),
// never EventError. It reports false when the context was cancelled while
// the enumeration was in flight.
func probeShares(ctx context.Context, h *model.Host, em *emitter, d deps) bool {
	type res struct {
		shares []model.Share
		err    error
	}
	ch := make(chan res, 1) // buffered: the abandoned goroutine never blocks
	go func() {
		s, err := d.listShares(ctx, h.IP)
		ch <- res{s, err}
	}()

	var r res
	select {
	case r = <-ch:
	case <-ctx.Done():
		return false
	}

	switch {
	case errors.Is(r.err, probe.ErrAccessDenied):
		deny := model.Share{Remark: "shares: access denied"}
		h.Shares = []model.Share{deny}
		em.send(Event{Kind: EventShareFound, Host: model.Host{IP: h.IP, Shares: []model.Share{deny}}})
	case r.err != nil:
		// Unreachable host, RPC failure, etc.: degrade silently.
	default:
		h.Shares = r.shares
		for _, s := range r.shares {
			em.send(Event{Kind: EventShareFound, Host: model.Host{IP: h.IP, Shares: []model.Share{s}}})
		}
	}
	return true
}

package scan

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/local/lancartographer/internal/hostcache"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
)

// runCollect runs run against a draining consumer and returns every emitted
// event in order plus run's error. The out channel is closed after run
// returns (run never sends after returning, so no send-on-close race) and
// the consumer is awaited, giving a happens-before edge for the result.
func runCollect(ctx context.Context, target netip.Prefix, opts probe.Options, d deps) ([]Event, error) {
	out := make(chan Event)
	drained := make(chan struct{})
	var evs []Event
	go func() {
		defer close(drained)
		for e := range out {
			evs = append(evs, e)
		}
	}()
	err := run(ctx, target, opts, out, d)
	close(out)
	<-drained
	return evs, err
}

// countKind returns how many events of kind k evs holds.
func countKind(evs []Event, k EventKind) int {
	n := 0
	for _, e := range evs {
		if e.Kind == k {
			n++
		}
	}
	return n
}

// lastOf returns the last event of kind k, or false when none was emitted.
func lastOf(evs []Event, k EventKind) (Event, bool) {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == k {
			return evs[i], true
		}
	}
	return Event{}, false
}

// fakeAliveDeps returns defaultDeps with the network-touching bindings
// replaced by fast fakes: every host is alive via tcp, ARP never matches,
// the real port scan is replaced by emitOpenPorts, and naming/MAC/shares
// are served from the given values. Name cache I/O is stubbed to empty.
func fakeAliveDeps(emitScan func(emit func(model.Port)), shares []model.Share, shareErr error, nb, mdns, mac string, macErr error, elevated bool) deps {
	d := defaultDeps(probe.Options{DialTimeout: time.Second})
	d.dial = func(context.Context, netip.Addr, int) bool { return true }
	d.arp = func(netip.Addr) bool { return false }
	d.icmp = nil
	if emitScan != nil {
		d.scan = func(_ context.Context, _ netip.Addr, _ probe.Options, _ *probe.Limiter, emit func(model.Port)) {
			emitScan(emit)
		}
	}
	d.listShares = func(context.Context, netip.Addr) ([]model.Share, error) { return shares, shareErr }
	d.nbName = func(context.Context, netip.Addr) string { return nb }
	d.mdnsName = func(context.Context, netip.Addr) string { return mdns }
	d.revName = func(context.Context, netip.Addr) string { return "REVSHOULDNOTWIN" }
	d.lookupMAC = func(netip.Addr) (string, error) { return mac, macErr }
	d.isElevated = func() bool { return elevated }
	d.oui = func(string) string { return "FakeVendor" }
	d.loadNames = func() map[string]hostcache.Entry { return map[string]hostcache.Entry{} }
	d.saveNames = func(map[string]hostcache.Entry) {}
	return d
}

func TestRun_SelectedPortOnly(t *testing.T) {
	// The host has no ARP entry and answers only the caller's selected port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	opts := probe.Options{Ports: []int{port}, DialTimeout: 100 * time.Millisecond, ProbeTimeout: 100 * time.Millisecond, Concurrency: 8}
	d := fakeAliveDeps(nil, nil, nil, "fixture", "", "", nil, true)
	realDial := tcpDial(opts.DialTimeout)
	d.dial = func(ctx context.Context, ip netip.Addr, p int) bool {
		return p == port && realDial(ctx, ip, p)
	}
	evs, err := runCollect(context.Background(), netip.MustParsePrefix("127.0.0.1/32"), opts, d)
	if err != nil {
		t.Fatal(err)
	}
	host, ok := lastOf(evs, EventHostFound)
	if !ok || host.Host.FoundVia != "tcp" || len(host.Host.Ports) != 1 || host.Host.Ports[0].State != model.PortOpen {
		t.Fatalf("selected-port-only host missing: %+v", evs)
	}
}

func TestRun_HostFoundEnriched(t *testing.T) {
	// Real loopback listener so the default (real) probe.ScanHost binding is
	// exercised: one genuinely open port, found by a real TCP connect.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	fixturePort := ln.Addr().(*net.TCPAddr).Port

	shares := []model.Share{
		{Name: "DATA", Type: model.ShareDisk, UNC: `\\127.0.0.1\DATA`},
		{Name: "PRINTQ", Type: model.SharePrint, UNC: `\\127.0.0.1\PRINTQ`},
	}
	d := fakeAliveDeps(nil, shares, nil, "TESTBOX", "", "3c:5a:b4:00:00:00", nil, true)
	// Wrap the real ScanHost (the fixture listener proves a genuine TCP
	// connect scan) and additionally report 445 open so the share probe runs.
	realScan := d.scan
	d.scan = func(ctx context.Context, ip netip.Addr, o probe.Options, lim *probe.Limiter, emit func(model.Port)) {
		realScan(ctx, ip, o, lim, emit)
		emit(model.Port{Number: 445, State: model.PortOpen, Service: "smb"})
	}

	opts := probe.Options{
		Ports:        []int{fixturePort},
		DialTimeout:  200 * time.Millisecond,
		ProbeTimeout: 500 * time.Millisecond,
		Concurrency:  8,
	}
	target := netip.MustParsePrefix("127.0.0.1/32")

	evs, err := runCollect(context.Background(), target, opts, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := countKind(evs, EventError); got != 0 {
		t.Fatalf("no EventError expected, got %d", got)
	}
	if got := countKind(evs, EventHostFound); got != 1 {
		t.Fatalf("exactly one EventHostFound expected, got %d", got)
	}
	if got := countKind(evs, EventPortFound); got != 2 {
		t.Fatalf("two EventPortFound expected (fixture + synthetic 445), got %d", got)
	}
	if got := countKind(evs, EventShareFound); got != 2 {
		t.Fatalf("two EventShareFound expected, got %d", got)
	}

	// Live events for the host must all precede its canonical HostFound.
	hostIdx := -1
	for i, e := range evs {
		if e.Kind == EventHostFound {
			hostIdx = i
		}
	}
	if hostIdx < 0 {
		t.Fatal("EventHostFound missing")
	}
	for i, e := range evs[:hostIdx] {
		if e.Kind != EventPortFound && e.Kind != EventShareFound && e.Kind != EventProgress {
			t.Fatalf("event %d before HostFound has kind %d, want live/port/share/progress only", i, e.Kind)
		}
	}

	hf := evs[hostIdx].Host
	if hf.IP != netip.MustParseAddr("127.0.0.1") {
		t.Errorf("IP = %v", hf.IP)
	}
	if !hf.Alive {
		t.Error("Alive = false")
	}
	if hf.FoundVia != "tcp" {
		t.Errorf("FoundVia = %q, want tcp", hf.FoundVia)
	}
	if hf.Name != "TESTBOX" {
		t.Errorf("Name = %q, want TESTBOX (NetBIOS beats reverse)", hf.Name)
	}
	if hf.MAC != "3c:5a:b4:00:00:00" {
		t.Errorf("MAC = %q", hf.MAC)
	}
	if hf.Vendor != "FakeVendor" {
		t.Errorf("Vendor = %q, want FakeVendor", hf.Vendor)
	}
	if len(hf.Ports) != 2 || hf.Ports[0].Number != 445 || hf.Ports[1].Number != fixturePort || hf.Ports[1].State != model.PortOpen {
		t.Errorf("Ports = %+v, want open 445 then open %d", hf.Ports, fixturePort)
	}
	if len(hf.Shares) != 2 {
		t.Errorf("Shares = %+v, want 2", hf.Shares)
	}

	// Every live event must carry the host IP and a single-item payload.
	for _, e := range evs[:hostIdx] {
		if e.Host.IP != hf.IP {
			t.Errorf("live event kind %d has IP %v", e.Kind, e.Host.IP)
		}
		if e.Kind == EventPortFound && len(e.Host.Ports) != 1 {
			t.Errorf("EventPortFound Ports = %+v, want exactly one", e.Host.Ports)
		}
		if e.Kind == EventShareFound && len(e.Host.Shares) != 1 {
			t.Errorf("EventShareFound Shares = %+v, want exactly one", e.Host.Shares)
		}
	}

	// Progress marches to 1/total and Done carries Progress=1 as the last event.
	if last := evs[len(evs)-1]; last.Kind != EventDone || last.Progress != 1 {
		t.Errorf("last event = kind %d progress %v, want Done/1", last.Kind, last.Progress)
	}
	if !lastOfProgressOK(evs, 1.0) {
		t.Error("no EventProgress with Progress=1.0 (single host completed)")
	}
}

func lastOfProgressOK(evs []Event, want float64) bool {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == EventProgress {
			return evs[i].Progress == want
		}
	}
	return false
}

func TestRun_AccessDeniedShareIsNotError(t *testing.T) {
	emitOpen := func(emit func(model.Port)) {
		emit(model.Port{Number: 445, State: model.PortOpen, Service: "smb"})
	}
	d := fakeAliveDeps(emitOpen, nil, probe.ErrAccessDenied, "BOX", "", "", errors.New("arp failed"), true)

	opts := probe.Options{Ports: []int{445}, Concurrency: 4}
	target := netip.MustParsePrefix("192.0.2.1/32")

	evs, err := runCollect(context.Background(), target, opts, d)
	if err != nil {
		t.Fatalf("run: %v (access denied must not fail the run)", err)
	}
	if got := countKind(evs, EventError); got != 0 {
		t.Fatalf("EventError count = %d, want 0 (R10: access denied is a share, not an error)", got)
	}
	if got := countKind(evs, EventShareFound); got != 1 {
		t.Fatalf("EventShareFound count = %d, want exactly the synthetic denial share", got)
	}
	sf, _ := lastOf(evs, EventShareFound)
	if len(sf.Host.Shares) != 1 {
		t.Fatalf("denial ShareFound carries %d shares, want 1", len(sf.Host.Shares))
	}
	s := sf.Host.Shares[0]
	if s.Remark != "shares: access denied" || s.Name != "" || s.Type != "" || s.UNC != "" {
		t.Errorf("synthetic share = %+v, want empty fields + denial remark", s)
	}
	// The canonical host carries the denial share too, and lookupMAC's error
	// with isElevated=true must NOT produce the elevation sentinel.
	hf, ok := lastOf(evs, EventHostFound)
	if !ok {
		t.Fatal("EventHostFound missing")
	}
	if len(hf.Host.Shares) != 1 || hf.Host.Shares[0].Remark != "shares: access denied" {
		t.Errorf("canonical shares = %+v", hf.Host.Shares)
	}
	if hf.Host.MAC != "" {
		t.Errorf("MAC = %q, want empty (elevated, lookup failed silently)", hf.Host.MAC)
	}
	if hf.Host.Vendor != "" {
		t.Errorf("Vendor = %q, want empty (no valid MAC)", hf.Host.Vendor)
	}
}

func TestRun_MACNeedsElevationWhenNotElevated(t *testing.T) {
	emitOpen := func(emit func(model.Port)) {
		emit(model.Port{Number: 22, State: model.PortOpen, Service: "ssh"})
	}
	d := fakeAliveDeps(emitOpen, nil, nil, "", "", "", errors.New("sendarp failed"), false)

	target := netip.MustParsePrefix("192.0.2.1/32")
	evs, err := runCollect(context.Background(), target, probe.Options{Ports: []int{22}}, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	hf, ok := lastOf(evs, EventHostFound)
	if !ok {
		t.Fatal("EventHostFound missing")
	}
	want := ""
	if runtime.GOOS == "windows" {
		want = probe.MacNeedsElevation
	}
	if hf.Host.MAC != want {
		t.Errorf("MAC = %q, want %q on %s", hf.Host.MAC, want, runtime.GOOS)
	}
	if hf.Host.Vendor != "" {
		t.Errorf("Vendor = %q, want empty for the sentinel (not a real MAC)", hf.Host.Vendor)
	}
}

func TestRun_ReverseNameFallback(t *testing.T) {
	emitOpen := func(emit func(model.Port)) {}
	d := fakeAliveDeps(emitOpen, nil, nil, "", "", "00:11:22:33:44:55", nil, true)

	target := netip.MustParsePrefix("192.0.2.1/32")
	evs, err := runCollect(context.Background(), target, probe.Options{Ports: []int{80}}, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	hf, ok := lastOf(evs, EventHostFound)
	if !ok {
		t.Fatal("EventHostFound missing")
	}
	if hf.Host.Name != "REVSHOULDNOTWIN" {
		t.Errorf("Name = %q, want reverse-DNS fallback when NetBIOS and mDNS are empty", hf.Host.Name)
	}
}

func TestRun_NamePriorityMDNSBeatsPTR(t *testing.T) {
	// NetBIOS empty, mDNS answered, PTR answered too: mDNS wins over PTR.
	emitOpen := func(emit func(model.Port)) {}
	d := fakeAliveDeps(emitOpen, nil, nil, "", "mdnsname", "00:11:22:33:44:55", nil, true)

	target := netip.MustParsePrefix("192.0.2.1/32")
	evs, err := runCollect(context.Background(), target, probe.Options{Ports: []int{80}}, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	hf, ok := lastOf(evs, EventHostFound)
	if !ok {
		t.Fatal("EventHostFound missing")
	}
	if hf.Host.Name != "mdnsname" {
		t.Errorf("Name = %q, want mDNS result ahead of reverse-DNS", hf.Host.Name)
	}
}

func TestRun_NameCacheFallback(t *testing.T) {
	// All probes come back empty; the MAC-keyed cache must fill the name.
	emitOpen := func(emit func(model.Port)) {}
	d := fakeAliveDeps(emitOpen, nil, nil, "", "", "00:11:22:33:44:55", nil, true)
	d.loadNames = func() map[string]hostcache.Entry {
		return map[string]hostcache.Entry{
			"00:11:22:33:44:55": {Name: "oldname", Last: time.Now().Unix()},
		}
	}
	d.revName = func(context.Context, netip.Addr) string { return "" }

	target := netip.MustParsePrefix("192.0.2.1/32")
	evs, err := runCollect(context.Background(), target, probe.Options{Ports: []int{80}}, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	hf, ok := lastOf(evs, EventHostFound)
	if !ok {
		t.Fatal("EventHostFound missing")
	}
	if hf.Host.Name != "oldname" {
		t.Errorf("Name = %q, want cached name when every probe is empty", hf.Host.Name)
	}
}

func TestRun_NameCacheWrite(t *testing.T) {
	// A resolved name is persisted: saveNames must receive a map keyed by
	// the lower-case MAC carrying the probed name.
	emitOpen := func(emit func(model.Port)) {}
	d := fakeAliveDeps(emitOpen, nil, nil, "newname", "", "00:11:22:33:44:55", nil, true)
	var saved map[string]hostcache.Entry
	d.saveNames = func(m map[string]hostcache.Entry) { saved = m }

	target := netip.MustParsePrefix("192.0.2.1/32")
	if _, err := runCollect(context.Background(), target, probe.Options{Ports: []int{80}}, d); err != nil {
		t.Fatalf("run: %v", err)
	}
	if saved == nil {
		t.Fatal("saveNames never called")
	}
	e, ok := saved["00:11:22:33:44:55"]
	if !ok || e.Name != "newname" || e.Last == 0 {
		t.Errorf("saved = %v, want 00:11:22:33:44:55 -> newname with a fresh timestamp", saved)
	}
}

func TestRun_CancelUnwindsFast(t *testing.T) {
	// 64 TEST-NET-1 hosts; every dial blocks until ctx cancellation. Stop
	// after 100 ms: the whole Run must unwind well under 1 s and report
	// context.Canceled. No real network is touched.
	var active sync.WaitGroup // dial goroutines still blocked
	d := defaultDeps(probe.Options{DialTimeout: time.Second})
	d.icmp = nil
	d.dial = func(ctx context.Context, _ netip.Addr, _ int) bool {
		active.Add(1)
		<-ctx.Done()
		active.Done()
		return false
	}
	d.arp = func(netip.Addr) bool { return false }
	d.scan = func(context.Context, netip.Addr, probe.Options, *probe.Limiter, func(model.Port)) {}
	d.listShares = func(context.Context, netip.Addr) ([]model.Share, error) { return nil, nil }
	d.nbName = func(context.Context, netip.Addr) string { return "" }
	d.mdnsName = func(context.Context, netip.Addr) string { return "" }
	d.revName = func(context.Context, netip.Addr) string { return "" }
	d.lookupMAC = func(netip.Addr) (string, error) { return "", nil }
	d.isElevated = func() bool { return true }
	d.oui = func(string) string { return "" }
	d.loadNames = func() map[string]hostcache.Entry { return map[string]hostcache.Entry{} }
	d.saveNames = func(map[string]hostcache.Entry) {}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	out := make(chan Event)
	go func() {
		for range out { // drain so sends never block
		}
	}()
	err := run(ctx, netip.MustParsePrefix("192.0.2.0/26"), probe.Options{Ports: []int{445}, Concurrency: 16}, out, d)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if elapsed > time.Second {
		t.Errorf("Run took %v to unwind after cancel, want < 1s", elapsed)
	}
	// Every blocked dial goroutine must have exited (poll: scheduling slack).
	waitDone := make(chan struct{})
	go func() {
		active.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Error("dial goroutines still blocked 1s after Run returned (leak)")
	}
}

func TestRun_DeadHosts(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	// Two dead hosts: no host/port/share events, progress marches 0.5 -> 1.
	d := defaultDeps(probe.Options{DialTimeout: time.Second})
	d.icmp = nil
	d.dial = func(context.Context, netip.Addr, int) bool { return false }
	d.arp = func(netip.Addr) bool { return false }

	opts := probe.Options{Ports: []int{445, 80}, Concurrency: 4}
	evs, err := runCollect(context.Background(), netip.MustParsePrefix("192.0.2.0/30"), opts, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, k := range []EventKind{EventHostFound, EventPortFound, EventShareFound, EventError} {
		if got := countKind(evs, k); got != 0 {
			t.Errorf("kind %d count = %d, want 0", k, got)
		}
	}
	var prog []float64
	for _, e := range evs {
		if e.Kind == EventProgress {
			prog = append(prog, e.Progress)
		}
	}
	if len(prog) != 2 || prog[0] != 0.5 || prog[1] != 1 {
		t.Errorf("progress = %v, want [0.5 1]", prog)
	}
	if last := evs[len(evs)-1]; last.Kind != EventDone || last.Progress != 1 {
		t.Errorf("last event = kind %d progress %v, want Done/1", last.Kind, last.Progress)
	}
}

func TestRun_ProgressOrdered(t *testing.T) {
	// Contending workers must deliver every completion in increasing order,
	// even when the consumer forces channel backpressure.
	d := fakeAliveDeps(nil, nil, nil, "", "", "", nil, true)
	d.dial = func(context.Context, netip.Addr, int) bool { return false }
	d.arp = func(netip.Addr) bool { return false }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	evs, err := runCollect(ctx, netip.MustParsePrefix("192.0.2.0/24"), probe.Options{Concurrency: 256}, d)
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, ev := range evs {
		if ev.Kind != EventProgress {
			continue
		}
		completed++
		if want := float64(completed) / 254; ev.Progress != want {
			t.Fatalf("completion %d: progress %v, want %v", completed, ev.Progress, want)
		}
	}
	if completed != 254 || evs[len(evs)-1].Kind != EventDone {
		t.Fatalf("incomplete stream: %d progress events", completed)
	}
}

func TestRun_StopDuringShareProbe(t *testing.T) {
	// Cancel while the share enumeration is blocked: prompt unwind, and the
	// abandoned listShares goroutine must exit via the per-host ctx (no leak).
	var blocked sync.WaitGroup
	blocked.Add(1)
	started := make(chan struct{})
	var once sync.Once

	emitOpen := func(emit func(model.Port)) {
		emit(model.Port{Number: 445, State: model.PortOpen, Service: "smb"})
	}
	d := fakeAliveDeps(emitOpen, nil, nil, "BOX", "", "", nil, true)
	d.listShares = func(ctx context.Context, _ netip.Addr) ([]model.Share, error) {
		once.Do(func() { close(started) })
		defer blocked.Done()
		<-ctx.Done()
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	start := time.Now()
	out := make(chan Event)
	go func() {
		for range out {
		}
	}()
	err := run(ctx, netip.MustParsePrefix("192.0.2.1/32"), probe.Options{Ports: []int{445}}, out, d)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if elapsed > time.Second {
		t.Errorf("Run took %v to unwind, want < 1s", elapsed)
	}
	waitDone := make(chan struct{})
	go func() {
		blocked.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Error("listShares goroutine still blocked 1s after Run returned (leak)")
	}
}

func TestRun_ICMPFallbackCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		d := deps{
			dial: func(context.Context, netip.Addr, int) bool { return false },
			arp:  func(netip.Addr) bool { return false },
			icmp: func(ctx context.Context, _ netip.Addr) bool {
				calls++
				<-ctx.Done()
				return false
			},
		}
		done := make(chan error, 1)
		go func() {
			_, err := runCollect(ctx, netip.MustParsePrefix("192.0.2.0/30"), probe.Options{Concurrency: 1}, d)
			done <- err
		}()
		synctest.Wait()
		if calls != 1 {
			t.Errorf("ICMP probes in flight = %d, want shared budget of 1", calls)
		}
		cancel()
		synctest.Wait()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("run = %v, want cancellation", err)
		}
	})
}

func TestRun_EmptyPrefix(t *testing.T) {
	// A prefix with no usable host addresses completes cleanly: just Done.
	d := fakeAliveDeps(nil, nil, nil, "", "", "", nil, true)
	evs, err := runCollect(context.Background(), netip.Prefix{}, probe.Options{}, d)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(evs) != 1 || evs[0].Kind != EventDone || evs[0].Progress != 1 {
		t.Errorf("events = %+v, want exactly Done/1", evs)
	}
}

func TestRun_EmptyPrefixCancellation(t *testing.T) {
	for _, when := range []string{"before run", "before run, buffered", "while blocked"} {
		t.Run(when, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if when != "while blocked" {
					cancel()
				}
				out := make(chan Event)
				if when == "before run, buffered" {
					out = make(chan Event, 1)
				}
				done := make(chan error, 1)
				go func() {
					done <- run(ctx, netip.Prefix{}, probe.Options{}, out, deps{})
				}()
				synctest.Wait()
				cancel()
				synctest.Wait()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("run = %v, want context.Canceled", err)
					}
				default:
					// Release the blocked sender before failing the regression.
					<-out
					<-done
					t.Fatal("run remained blocked sending Done after cancellation")
				}
			})
		})
	}
}

func TestValidateTarget(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{"ipv6 prefix rejected", "2001:db8::/64", true},
		{"too large /8 rejected", "10.0.0.0/8", true},
		{"boundary /12 allowed", "10.0.0.0/12", false},
		{"typical /24 allowed", "192.168.1.0/24", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTarget(netip.MustParsePrefix(tc.target))
			if tc.wantErr && err == nil {
				t.Fatalf("validateTarget(%s) = nil, want error", tc.target)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateTarget(%s) = %v, want nil", tc.target, err)
			}
		})
	}
	// A zero prefix stays a clean no-op (existing empty-scan behavior).
	if err := validateTarget(netip.Prefix{}); err != nil {
		t.Errorf("validateTarget(zero) = %v, want nil", err)
	}
}

func TestResolveNameStopsOnceABetterNameExists(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.10")
	block := func(ctx context.Context, _ netip.Addr) string {
		<-ctx.Done()
		return "late"
	}
	d := deps{
		nbName:   func(context.Context, netip.Addr) string { return "nbhost" },
		mdnsName: block,
		revName:  block,
	}
	start := time.Now()
	if got := resolveName(context.Background(), ip, d); got != "nbhost" {
		t.Fatalf("resolveName = %q, want nbhost", got)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("resolveName waited %s after NetBIOS had the name", elapsed)
	}

	d.nbName = func(context.Context, netip.Addr) string { return "" }
	d.mdnsName = func(context.Context, netip.Addr) string { return "mdnshost" }
	start = time.Now()
	if got := resolveName(context.Background(), ip, d); got != "mdnshost" {
		t.Fatalf("resolveName = %q, want mdnshost", got)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("resolveName waited %s after mDNS had the name", elapsed)
	}
}

func TestRun_RejectsInvalidTargets(t *testing.T) {
	d := fakeAliveDeps(nil, nil, nil, "", "", "", nil, true)
	for _, target := range []string{"2001:db8::/64", "10.0.0.0/8"} {
		evs, err := runCollect(context.Background(), netip.MustParsePrefix(target), probe.Options{}, d)
		if err == nil {
			t.Errorf("run(%s) = nil error, want validation failure", target)
		}
		if len(evs) != 0 {
			t.Errorf("run(%s) emitted %d events, want none on validation failure", target, len(evs))
		}
	}
}

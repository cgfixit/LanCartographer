// Package cli is LanCartographer's table interface: it parses flags,
// applies the authorization gate, drives the scan engine (one --range
// target, or every local subnet when --range is omitted), and prints a
// table or --json - JSON of the merged inventory to stdout. All diagnostics go
// to stderr. Exit codes: 0 success; 2 invalid input, authorization refusal,
// scan failure or JSON export failure; 3 no usable local interfaces on the
// omitted --range path; 4 completed scan with unmet expectations.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/local/lancartographer/internal/authz"
	"github.com/local/lancartographer/internal/discover"
	"github.com/local/lancartographer/internal/export"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
	"github.com/local/lancartographer/internal/scan"
)

// deps is the testability seam (same pattern as the scan engine): every
// external effect the CLI needs binds here, with Run wiring the real
// implementations.
type deps struct {
	localSubnets      func(includeVirtual bool) ([]discover.InterfaceInfo, error)
	scanRun           func(ctx context.Context, target netip.Prefix, opts probe.Options, out chan<- scan.Event) error
	exportToFile      func(path string, hosts []model.Host, format string) error
	needsConfirmation func(target netip.Prefix, local []netip.Prefix) bool
}

// Run is the CLI entry point used by cmd/lancartographer. It binds the
// real subsystem functions and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, deps{
		localSubnets:      discover.LocalSubnets,
		scanRun:           scan.Run,
		exportToFile:      export.ToFile,
		needsConfirmation: authz.NeedsConfirmation,
	})
}

func run(args []string, stdout, stderr io.Writer, d deps) int {
	fs := flag.NewFlagSet("lancartographer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rangeArg := fs.String("range", "", "CIDR range to scan (default: every local physical subnet)")
	portsArg := fs.String("ports", probe.DefaultPortsCSV, "comma-separated TCP ports to probe")
	timeout := fs.Duration("timeout", 400*time.Millisecond,
		"per-host dial timeout; the per-dial probe timeout is 2x this value")
	concurrency := fs.Int("concurrency", 128, "dial concurrency (engine clamps to 256)")
	jsonPath := fs.String("json", "", "also write JSON to PATH; use - for JSON-only stdout")
	var expectPath string
	fs.Func("expect", "check scan results against a versioned JSON `file`", func(path string) error {
		if path == "" {
			return fmt.Errorf("file path must not be empty")
		}
		expectPath = path
		return nil
	})
	authorized := fs.Bool("i-am-authorized", false, "confirm authorization for ranges that require it")
	if err := fs.Parse(args); err != nil {
		return 2 // flag package already reported to stderr
	}

	// net.Dialer treats a zero timeout as none (dials wait for the OS
	// connect timeout) and a negative one as already expired (every dial
	// fails, so live hosts vanish from the inventory).
	if *timeout <= 0 {
		fmt.Fprintf(stderr, "lancartographer: --timeout must be greater than zero, got %s\n", *timeout)
		return 2
	}

	ports, err := probe.ParsePorts(*portsArg)
	if err != nil {
		fmt.Fprintf(stderr, "lancartographer: --ports: %v\n", err)
		return 2
	}

	// Local subnets drive both the confirmation gate and, when --range is
	// omitted, the scan target list.
	ifs, err := d.localSubnets(false)
	if err != nil && *rangeArg == "" {
		fmt.Fprintf(stderr, "lancartographer: cannot enumerate local interfaces: %v\n", err)
		return 3
	}
	var local []netip.Prefix
	for _, inf := range ifs {
		local = append(local, inf.Prefix.Masked())
	}

	var targets []netip.Prefix
	if *rangeArg != "" {
		p, err := netip.ParsePrefix(*rangeArg)
		if err != nil {
			fmt.Fprintf(stderr, "lancartographer: --range %q is not a valid CIDR: %v\n", *rangeArg, err)
			return 2
		}
		targets = []netip.Prefix{p.Masked()}
	} else {
		if len(local) == 0 {
			fmt.Fprintln(stderr, "lancartographer: no local interfaces found; pass --range CIDR explicitly")
			return 3
		}
		targets = local
	}

	// Confirmation gate: check EVERY target before scanning anything, so a
	// single refusal exits 2 with nothing scanned. Local subnets are
	// contained by definition, but the /22 breadth rule can still fire.
	for _, target := range targets {
		if d.needsConfirmation(target, local) && !*authorized {
			fmt.Fprintln(stderr, authz.Notice)
			fmt.Fprintln(stderr)
			fmt.Fprintf(stderr, "Range %s requires confirmation. Re-run with --i-am-authorized if you have explicit authorization.\n", target)
			return 2
		}
	}

	var expected []hostExpectation
	if expectPath != "" {
		expected, err = loadExpectations(expectPath, *jsonPath, targets, ports)
		if err != nil {
			fmt.Fprintf(stderr, "lancartographer: --expect: %v\n", err)
			return 2
		}
	}

	// --timeout sets the liveness DialTimeout; the port-probe timeout is
	// 2x, matching the engine's 400ms/800ms defaults.
	opts := probe.Options{
		Ports:        ports,
		DialTimeout:  *timeout,
		ProbeTimeout: 2 * *timeout,
		Concurrency:  *concurrency,
	}

	// Merge across subnets deduped by IP: the first subnet to report a
	// host wins; later duplicates are dropped.
	byIP := make(map[netip.Addr]model.Host)
	for _, target := range targets {
		hosts, err := scanOne(target, opts, d.scanRun)
		if err != nil {
			fmt.Fprintf(stderr, "lancartographer: scan of %s failed: %v\n", target, err)
			return 2
		}
		for _, h := range hosts {
			if _, seen := byIP[h.IP]; !seen {
				byIP[h.IP] = h
			}
		}
	}
	hosts := make([]model.Host, 0, len(byIP))
	for _, h := range byIP {
		hosts = append(hosts, h)
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].IP.Compare(hosts[j].IP) < 0 })

	if *jsonPath == "-" {
		if err := model.WriteJSON(stdout, hosts); err != nil {
			fmt.Fprintf(stderr, "lancartographer: JSON stdout failed: %v\n", err)
			return 2
		}
	} else {
		printTable(stdout, hosts)
		if *jsonPath != "" {
			if err := d.exportToFile(*jsonPath, hosts, "json"); err != nil {
				fmt.Fprintf(stderr, "lancartographer: --json export to %s failed: %v\n", *jsonPath, err)
				return 2
			}
		}
	}

	if !reportExpectations(stderr, expected, byIP) {
		return 4
	}
	return 0
}

// scanOne runs one sweep, draining the event channel in a goroutine, and
// returns the canonical EventHostFound hosts only after completion.
func scanOne(target netip.Prefix, opts probe.Options, scanRun func(context.Context, netip.Prefix, probe.Options, chan<- scan.Event) error) ([]model.Host, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan scan.Event)
	done := make(chan []model.Host, 1)
	var completed bool
	go func() {
		var hosts []model.Host
		for ev := range events {
			switch ev.Kind {
			case scan.EventHostFound:
				hosts = append(hosts, ev.Host)
			case scan.EventDone:
				completed = ev.Progress == 1
			}
		}
		done <- hosts
	}()

	err := scanRun(ctx, target, opts, events)
	close(events) // Run never sends after returning
	hosts := <-done
	if err == nil && !completed {
		err = fmt.Errorf("scan ended without a completion event")
	}
	return hosts, err
}

// printTable writes the inventory table: IP, NAME, MAC, VENDOR, PORTS,
// SHARES, one row per host (already sorted by the caller).
func printTable(w io.Writer, hosts []model.Host) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "IP\tNAME\tMAC\tVENDOR\tPORTS\tSHARES")
	for _, h := range hosts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			tableCell(h.IP.String()), tableCell(h.Name), tableCell(h.MAC), tableCell(h.Vendor),
			tableCell(h.OpenPortsSummary()), tableCell(sharesCell(h)))
	}
	tw.Flush()
}

// tableCell keeps untrusted controls out of both the terminal and tabwriter.
func tableCell(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '\uFFFD'
		}
		return r
	}, s)
}

// sharesCell renders each share as name(type) plus :remark when the
// remark is non-empty, joined by ", ". Shares with an empty Name are
// skipped (the engine's synthetic access-denied record); a host with
// ONLY that record prints "shares: access denied".
func sharesCell(h model.Host) string {
	var parts []string
	for _, s := range h.Shares {
		if s.Name == "" {
			continue
		}
		part := s.Name
		if s.Type != "" {
			part += "(" + s.Type + ")"
		}
		if s.Remark != "" {
			part += ":" + s.Remark
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		if len(h.Shares) > 0 {
			return "shares: access denied"
		}
		return ""
	}
	return strings.Join(parts, ", ")
}

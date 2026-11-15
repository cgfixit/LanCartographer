// Package servicecheck diagnoses one operator-selected service endpoint:
// it resolves a name through the system resolver, then dials one pinned
// address and reports separate connect, TLS and HTTP stages.
//
// The package has no UI and no global state. It never follows redirects,
// never uses a proxy, never disables TLS verification and never re-resolves
// the name after the operator has chosen an address. Callers own the
// authorization step; Check contacts exactly the address it is given.
package servicecheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Protocol selects how far Check goes after the TCP connection.
type Protocol string

const (
	// TCP stops after the connection is established.
	TCP Protocol = "TCP"
	// TLS adds a verified TLS handshake.
	TLS Protocol = "TLS"
	// HTTP sends one plain GET / on the connection.
	HTTP Protocol = "HTTP"
	// HTTPS sends one GET / after a verified TLS handshake.
	HTTPS Protocol = "HTTPS"
)

// Protocols lists the selectable protocols in display order.
var Protocols = []Protocol{TCP, TLS, HTTP, HTTPS}

func (p Protocol) valid() bool    { return p == TCP || p == TLS || p == HTTP || p == HTTPS }
func (p Protocol) usesTLS() bool  { return p == TLS || p == HTTPS }
func (p Protocol) usesHTTP() bool { return p == HTTP || p == HTTPS }

// SuggestProtocol returns a reasonable default protocol for a port. It is
// a prefill only; the operator chooses before anything is contacted.
func SuggestProtocol(port int) Protocol {
	switch port {
	case 443, 5001, 8443, 9443:
		return HTTPS
	case 80, 3000, 5000, 5173, 8000, 8008, 8080, 8888:
		return HTTP
	case 465, 636, 853, 993, 995:
		return TLS
	}
	return TCP
}

// Limits. The HTTP limits match the inventory probe (internal/probe).
const (
	ResolveTimeout = 5 * time.Second
	ConnectTimeout = 3 * time.Second
	TLSTimeout     = 3 * time.Second
	HTTPTimeout    = time.Second
	MaxAddresses   = 16
	maxNameLen     = 253
	detailMaxRunes = 160
)

// Outcome is a stage verdict.
type Outcome int

const (
	// Skipped means the stage did not run (not requested, or an earlier
	// stage failed).
	Skipped Outcome = iota
	// Passed means the stage completed as expected.
	Passed
	// Warning means the stage completed but the answer needs attention
	// (for example an HTTP 5xx or a certificate close to expiry).
	Warning
	// Failed means the stage could not complete.
	Failed
)

func (o Outcome) String() string {
	switch o {
	case Passed:
		return "Passed"
	case Warning:
		return "Warning"
	case Failed:
		return "Failed"
	}
	return "Skipped"
}

// Stage is one step of a check.
type Stage struct {
	Name    string
	Outcome Outcome
	Detail  string
	Elapsed time.Duration
}

// Target is the confirmed endpoint. Host is the name as entered (used for
// TLS verification and the HTTP Host header); Addr is the only address
// dialed.
type Target struct {
	Host     string
	Addr     netip.Addr
	Port     uint16
	Protocol Protocol
}

// Endpoint renders the dialed address and port.
func (t Target) Endpoint() string { return netip.AddrPortFrom(t.Addr, t.Port).String() }

func (t Target) isLiteral() bool {
	a, err := netip.ParseAddr(strings.Trim(t.Host, "[]"))
	return err == nil && a.WithZone("") == t.Addr.WithZone("")
}

// Result is the observation of one check.
type Result struct {
	Target    Target
	Started   time.Time
	Stages    []Stage
	Cancelled bool
}

// Outcome is the worst outcome across the stages that ran.
func (r Result) Outcome() Outcome {
	worst := Skipped
	for _, s := range r.Stages {
		if s.Outcome > worst {
			worst = s.Outcome
		}
	}
	return worst
}

// Text renders a plain-text summary for an explicit copy request.
func (r Result) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "LanCartographer service check · %s\n", r.Started.Format("2006-01-02 15:04:05 MST"))
	target := r.Target.Endpoint()
	if !r.Target.isLiteral() {
		target = r.Target.Host + " → " + target
	}
	fmt.Fprintf(&b, "Target  %s (%s)\n", target, r.Target.Protocol)
	for _, s := range r.Stages {
		fmt.Fprintf(&b, "%-8s %-8s %s", s.Name, s.Outcome, s.Detail)
		if s.Outcome != Skipped {
			fmt.Fprintf(&b, " (%s)", FormatElapsed(s.Elapsed))
		}
		b.WriteByte('\n')
	}
	if r.Cancelled {
		b.WriteString("Cancelled before completion.\n")
	}
	b.WriteString("Observed from this computer only; it does not establish what other clients see.\n")
	return b.String()
}

func seconds(d time.Duration) string { return fmt.Sprintf("%d s", int(d/time.Second)) }

// FormatElapsed renders a stage duration with millisecond precision.
func FormatElapsed(d time.Duration) string {
	if d < time.Millisecond {
		return "<1 ms"
	}
	return fmt.Sprintf("%d ms", d.Milliseconds())
}

// ParseHost validates operator input. It returns the host to keep for TLS
// and HTTP, and the address when the input is a literal. A link-local IPv6
// literal must carry a zone naming a current interface (ifaceExists).
func ParseHost(input string, ifaceExists func(string) bool) (host string, literal netip.Addr, err error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", netip.Addr{}, errors.New("Enter a host name or IP address.")
	}
	if len(s) > maxNameLen+2 {
		return "", netip.Addr{}, errors.New("That host name is too long.")
	}
	if strings.Contains(s, "://") || strings.ContainsAny(s, "/?#@ \t") {
		return "", netip.Addr{}, errors.New("Enter only a host name or address, without a scheme, path or spaces.")
	}
	bracketed := strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")
	if bracketed {
		s = s[1 : len(s)-1]
	}
	if addr, perr := netip.ParseAddr(s); perr == nil {
		if err := checkLiteral(addr, ifaceExists); err != nil {
			return "", netip.Addr{}, err
		}
		addr = unmap(addr)
		return addr.String(), addr, nil
	}
	if strings.Contains(s, "%") {
		return "", netip.Addr{}, errors.New("An interface zone is only valid on a link-local IPv6 address.")
	}
	if bracketed || strings.Contains(s, ":") {
		return "", netip.Addr{}, errors.New("Enter the port in the Port field, or a valid IPv6 address.")
	}
	if err := checkHostname(s); err != nil {
		return "", netip.Addr{}, err
	}
	return s, netip.Addr{}, nil
}

func unmap(a netip.Addr) netip.Addr {
	if a.Is4In6() {
		return a.Unmap()
	}
	return a
}

func checkLiteral(a netip.Addr, ifaceExists func(string) bool) error {
	if a.Is4In6() && a.Zone() != "" {
		return errors.New("An interface zone is only valid on a link-local IPv6 address.")
	}
	a = unmap(a)
	if !a.IsValid() || a.IsUnspecified() || a.IsMulticast() || a == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return errors.New("Enter one unicast host address.")
	}
	zone := a.Zone()
	if a.Is6() && a.IsLinkLocalUnicast() {
		if zone == "" {
			return errors.New("A link-local IPv6 address needs its interface, for example fe80::1%en0.")
		}
		if clean(zone, 16) != zone {
			return errors.New("That interface name is not valid.")
		}
		if ifaceExists == nil || !ifaceExists(zone) {
			return fmt.Errorf("Interface %q is not present on this computer.", zone)
		}
		return nil
	}
	if zone != "" {
		return errors.New("An interface zone is only valid on a link-local IPv6 address.")
	}
	return nil
}

func checkHostname(s string) error {
	name := strings.TrimSuffix(s, ".")
	if name == "" || len(name) > maxNameLen {
		return errors.New("Enter a valid host name.")
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("Enter a valid host name.")
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return errors.New("Enter a valid host name (letters, digits, hyphens and dots).")
			}
		}
	}
	last := labels[len(labels)-1]
	if strings.Trim(last, "0123456789") == "" {
		return errors.New("That is not a valid IP address.")
	}
	return nil
}

// Resolver is the lookup seam; *net.Resolver satisfies it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Resolve looks up host once through r (the system resolver when nil),
// bounded by ResolveTimeout. It returns at most MaxAddresses distinct,
// dialable addresses and a Resolve stage describing the outcome.
func Resolve(ctx context.Context, r Resolver, host string, ifaceExists func(string) bool) ([]netip.Addr, Stage) {
	if r == nil {
		r = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(ctx, ResolveTimeout)
	defer cancel()
	start := time.Now()
	found, err := r.LookupNetIP(ctx, "ip", host)
	stage := Stage{Name: "Resolve", Elapsed: time.Since(start)}
	if err != nil {
		stage.Outcome = Failed
		stage.Detail = resolveError(err)
		return nil, stage
	}
	seen := map[netip.Addr]bool{}
	var addrs []netip.Addr
	omitted := 0
	for _, a := range found {
		a = unmap(a)
		if seen[a] {
			continue
		}
		seen[a] = true
		if checkLiteral(a, ifaceExists) != nil || len(addrs) == MaxAddresses {
			omitted++
			continue
		}
		addrs = append(addrs, a)
	}
	if len(addrs) == 0 {
		stage.Outcome = Failed
		stage.Detail = "The name resolved, but to no address this computer can connect to."
		return nil, stage
	}
	stage.Outcome = Passed
	list := make([]string, 0, 3)
	for i, a := range addrs {
		if i == 3 {
			list = append(list, "…")
			break
		}
		list = append(list, a.String())
	}
	noun := "address"
	if len(addrs) != 1 {
		noun = "addresses"
	}
	stage.Detail = fmt.Sprintf("%d %s: %s", len(addrs), noun, strings.Join(list, ", "))
	if omitted > 0 {
		stage.Detail += fmt.Sprintf(" (%d omitted)", omitted)
	}
	return addrs, stage
}

func resolveError(err error) string {
	var dnsErr *net.DNSError
	switch {
	case errors.Is(err, context.Canceled):
		return "Cancelled."
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return "Name not found. Check the spelling, the search domain, or whether this network's DNS knows the name."
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &dnsErr) && dnsErr.IsTimeout:
		return fmt.Sprintf("The resolver did not answer within %s.", seconds(ResolveTimeout))
	}
	return "Lookup failed: " + clean(err.Error(), detailMaxRunes)
}

// clean makes untrusted text (server headers, certificate names, error
// strings) safe to display on one line: invalid UTF-8, control and format
// characters (including bidirectional overrides) become spaces, runs of
// spaces collapse, and the result is truncated to max runes.
func clean(s string, max int) string {
	s = strings.ToValidUTF8(s, "�")
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return ' '
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max-1]) + "…"
	}
	return s
}

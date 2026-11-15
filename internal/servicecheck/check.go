package servicecheck

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/local/lancartographer/internal/probe"
)

const (
	// httpBodyCap matches the inventory probe's 32 KiB body limit.
	httpBodyCap = 32 << 10
	// httpHeaderCap bounds the status line and headers read before the body.
	httpHeaderCap = 32 << 10
	// expirySoon flags certificates close to expiry.
	expirySoon = 14 * 24 * time.Hour
)

// Options are test seams. The zero value uses the system dialer, the
// system trust store and the package time limits.
type Options struct {
	Dial    func(ctx context.Context, network, address string) (net.Conn, error)
	RootCAs *x509.CertPool
	Now     func() time.Time
}

// Check dials t.Addr once and runs the stages t.Protocol requests. It
// returns an error only for an invalid target; every network outcome is a
// stage. The context bounds the whole check; cancelling it closes the
// connection and marks the result Cancelled.
func Check(ctx context.Context, t Target, opts Options) (Result, error) {
	if !t.Addr.IsValid() || t.Port == 0 || !t.Protocol.valid() || t.Host == "" {
		return Result{}, errors.New("servicecheck: incomplete target")
	}
	if opts.Dial == nil {
		d := &net.Dialer{}
		opts.Dial = d.DialContext
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	r := Result{Target: t, Started: opts.Now()}

	conn, stage := connect(ctx, t, opts)
	r.Stages = append(r.Stages, stage)
	if conn == nil {
		r.Stages = appendSkipped(r.Stages, t.Protocol, "Not attempted: no connection.")
		r.Cancelled = ctx.Err() != nil
		return r, nil
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if t.Protocol.usesTLS() {
		tlsConn, stage := handshake(ctx, conn, t, opts)
		r.Stages = append(r.Stages, stage)
		if tlsConn == nil {
			if t.Protocol.usesHTTP() {
				r.Stages = append(r.Stages, Stage{Name: "HTTP", Outcome: Skipped, Detail: "Not attempted: TLS did not complete."})
			}
			r.Cancelled = ctx.Err() != nil
			return r, nil
		}
		conn = tlsConn
	}
	if t.Protocol.usesHTTP() {
		r.Stages = append(r.Stages, request(conn, t))
	}
	r.Cancelled = ctx.Err() != nil
	return r, nil
}

func appendSkipped(stages []Stage, p Protocol, why string) []Stage {
	if p.usesTLS() {
		stages = append(stages, Stage{Name: "TLS", Outcome: Skipped, Detail: why})
	}
	if p.usesHTTP() {
		stages = append(stages, Stage{Name: "HTTP", Outcome: Skipped, Detail: why})
	}
	return stages
}

func connect(ctx context.Context, t Target, opts Options) (net.Conn, Stage) {
	stage := Stage{Name: "Connect"}
	dctx, cancel := context.WithTimeout(ctx, ConnectTimeout)
	defer cancel()
	start := time.Now()
	// A literal address: the dialer performs no name lookup and no proxy
	// is consulted, so the confirmed address is the only one contacted.
	conn, err := opts.Dial(dctx, "tcp", t.Endpoint())
	stage.Elapsed = time.Since(start)
	if err != nil {
		stage.Outcome = Failed
		stage.Detail = connectError(ctx, err, t.Port)
		return nil, stage
	}
	stage.Outcome = Passed
	stage.Detail = "Connected to " + t.Endpoint()
	if local, ok := conn.LocalAddr().(*net.TCPAddr); ok {
		stage.Detail += " from " + local.String()
	}
	return conn, stage
}

func connectError(parent context.Context, err error, port uint16) string {
	var nerr net.Error
	switch {
	case parent.Err() != nil:
		return "Cancelled."
	case isRefused(err):
		return fmt.Sprintf("Refused. Nothing is listening on port %d at this address, or a firewall rejected the connection.", port)
	case isUnreachable(err):
		return "No route to this address from this computer."
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &nerr) && nerr.Timeout():
		return fmt.Sprintf("No answer within %s. The host may be offline, a firewall may be dropping the connection, or the address may be wrong.", seconds(ConnectTimeout))
	}
	return "Could not connect: " + clean(err.Error(), detailMaxRunes)
}

// serverName is the name TLS verifies: the host as entered, or the IP
// literal (Go then checks IP SANs and sends no SNI).
func serverName(t Target) string {
	if t.isLiteral() {
		return t.Addr.WithZone("").String()
	}
	return strings.TrimSuffix(t.Host, ".")
}

func handshake(ctx context.Context, conn net.Conn, t Target, opts Options) (net.Conn, Stage) {
	stage := Stage{Name: "TLS"}
	cfg := &tls.Config{
		ServerName: serverName(t),
		RootCAs:    opts.RootCAs,
		MinVersion: tls.VersionTLS12,
		Time:       opts.Now,
	}
	if t.Protocol == HTTPS {
		cfg.NextProtos = []string{"http/1.1"}
	}
	tc := tls.Client(conn, cfg)
	_ = conn.SetDeadline(time.Now().Add(TLSTimeout))
	start := time.Now()
	err := tc.HandshakeContext(ctx)
	stage.Elapsed = time.Since(start)
	_ = conn.SetDeadline(time.Time{})
	if err != nil {
		stage.Outcome = Failed
		stage.Detail = tlsError(ctx, err, serverName(t), opts.Now())
		return nil, stage
	}
	state := tc.ConnectionState()
	stage.Outcome = Passed
	stage.Detail = tls.VersionName(state.Version)
	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		stage.Detail += " · verified certificate for " + certNames(leaf) + ", issued by " + certIssuer(leaf)
		left := leaf.NotAfter.Sub(opts.Now())
		stage.Detail += ", expires " + leaf.NotAfter.UTC().Format("2006-01-02")
		if left < expirySoon {
			stage.Outcome = Warning
			stage.Detail += fmt.Sprintf(" (in %d days)", int(left.Hours()/24))
		}
	}
	return tc, stage
}

func tlsError(parent context.Context, err error, name string, now time.Time) string {
	var (
		hostErr    x509.HostnameError
		authErr    x509.UnknownAuthorityError
		invalidErr x509.CertificateInvalidError
		recordErr  tls.RecordHeaderError
		nerr       net.Error
	)
	switch {
	case parent.Err() != nil:
		return "Cancelled."
	case errors.As(err, &hostErr) && hostErr.Certificate != nil:
		return fmt.Sprintf("The certificate is not valid for %s; it names %s.", clean(name, 64), certNames(hostErr.Certificate))
	case errors.As(err, &authErr):
		detail := "The certificate is not trusted by this computer"
		if authErr.Cert != nil {
			detail += " (issued by " + certIssuer(authErr.Cert) + ")"
		}
		return detail + ". Self-signed and private-CA certificates fail here; the app does not bypass verification."
	case errors.As(err, &invalidErr) && invalidErr.Reason == x509.Expired && invalidErr.Cert != nil:
		c := invalidErr.Cert
		if now.Before(c.NotBefore) {
			return "The certificate is not valid until " + c.NotBefore.UTC().Format("2006-01-02") + ". Check this computer's clock."
		}
		return "The certificate expired on " + c.NotAfter.UTC().Format("2006-01-02") + "."
	case errors.As(err, &recordErr):
		return "The service did not answer with TLS. It may be plain TCP or HTTP; try that protocol."
	case errors.Is(err, context.DeadlineExceeded) || errors.As(err, &nerr) && nerr.Timeout():
		return fmt.Sprintf("No TLS answer within %s.", seconds(TLSTimeout))
	case errors.Is(err, io.EOF) || isReset(err):
		return "The service closed the connection during the TLS handshake."
	}
	return "Handshake failed: " + clean(err.Error(), detailMaxRunes)
}

func certNames(c *x509.Certificate) string {
	names := append([]string{}, c.DNSNames...)
	for _, ip := range c.IPAddresses {
		names = append(names, ip.String())
	}
	if len(names) == 0 && c.Subject.CommonName != "" {
		names = append(names, c.Subject.CommonName)
	}
	if len(names) == 0 {
		return "no host names"
	}
	if len(names) > 3 {
		names = append(names[:3], fmt.Sprintf("+%d more", len(names)-3))
	}
	return clean(strings.Join(names, ", "), 96)
}

func certIssuer(c *x509.Certificate) string {
	issuer := c.Issuer.CommonName
	if issuer == "" && len(c.Issuer.Organization) > 0 {
		issuer = c.Issuer.Organization[0]
	}
	if issuer == "" {
		return "an unnamed issuer"
	}
	return clean(issuer, 64)
}

// hostHeader is the HTTP Host value: the entered name (or bracketed literal
// without a zone) plus the port when it is not the scheme default.
func hostHeader(t Target) string {
	host := serverName(t)
	if t.isLiteral() && t.Addr.Is6() {
		host = "[" + host + "]"
	}
	if (t.Protocol == HTTP && t.Port == 80) || (t.Protocol == HTTPS && t.Port == 443) {
		return host
	}
	return host + ":" + strconv.Itoa(int(t.Port))
}

// request writes one GET / on the already-established connection and reads
// a bounded response. No transport is involved, so no proxy, connection
// reuse, redirect or second dial can occur.
func request(conn net.Conn, t Target) (stage Stage) {
	stage.Name = "HTTP"
	scheme := "http"
	if t.Protocol == HTTPS {
		scheme = "https"
	}
	host := hostHeader(t)
	req, err := http.NewRequest(http.MethodGet, scheme+"://"+host+"/", nil)
	if err != nil {
		stage.Outcome = Failed
		stage.Detail = "Could not build the request."
		return stage
	}
	req.Host = host
	req.Close = true
	req.Header.Set("User-Agent", "LanCartographer-service-check")
	req.Header.Set("Accept", "*/*")

	_ = conn.SetDeadline(time.Now().Add(HTTPTimeout))
	start := time.Now()
	defer func() { stage.Elapsed = time.Since(start) }()
	if err := req.Write(conn); err != nil {
		stage.Outcome = Failed
		stage.Detail = httpError(err)
		return stage
	}
	limited := &io.LimitedReader{R: conn, N: httpHeaderCap}
	br := bufio.NewReader(limited)
	resp, err := http.ReadResponse(br, req)
	// Skip interim 1xx answers (100 Continue, 103 Early Hints) within the
	// same deadline and byte cap; report the final status.
	for err == nil && resp.StatusCode >= 100 && resp.StatusCode < 200 && resp.StatusCode != http.StatusSwitchingProtocols {
		resp.Body.Close()
		resp, err = http.ReadResponse(br, req)
	}
	if err != nil {
		stage.Outcome = Failed
		if limited.N == 0 {
			stage.Detail = "The response headers exceeded 32 KiB."
		} else {
			stage.Detail = httpError(err)
		}
		return stage
	}
	defer resp.Body.Close()
	// Headers are read; allow the body its own 32 KiB on top of whatever
	// the bufio reader already buffered.
	limited.N = httpBodyCap
	body, _ := io.ReadAll(io.LimitReader(resp.Body, httpBodyCap))

	stage.Outcome = Passed
	stage.Detail = "HTTP " + clean(resp.Status, 64)
	if server := clean(resp.Header.Get("Server"), 48); server != "" {
		stage.Detail += " · server " + server
	}
	if title := probe.ExtractTitle(string(body)); title != "" {
		stage.Detail += " · “" + clean(title, 80) + "”"
	}
	switch {
	case resp.StatusCode == http.StatusSwitchingProtocols:
		stage.Outcome = Warning
		stage.Detail += " · the service switched protocols instead of answering the request"
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		if loc := clean(resp.Header.Get("Location"), 96); loc != "" {
			stage.Detail += " · redirect to " + loc + " (not followed)"
		}
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		stage.Detail += " · the service answered and requires sign-in or denies access"
	case resp.StatusCode >= 500:
		stage.Outcome = Warning
		stage.Detail += " · the service answered with a server error"
	}
	return stage
}

func httpError(err error) string {
	var nerr net.Error
	switch {
	case errors.As(err, &nerr) && nerr.Timeout():
		return fmt.Sprintf("No HTTP response within %s.", seconds(HTTPTimeout))
	case errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || isReset(err):
		return "The service closed the connection without an HTTP response."
	case strings.Contains(err.Error(), "malformed HTTP"):
		return "The service did not answer with HTTP. It may use another protocol or require TLS."
	case strings.Contains(err.Error(), "malformed"):
		return "The service sent a response that is not valid HTTP."
	}
	return "Request failed: " + clean(err.Error(), detailMaxRunes)
}

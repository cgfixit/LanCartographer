package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"html"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

const (
	// httpProbeTimeout caps every title probe, regardless of any longer
	// deadline on the caller's context.
	httpProbeTimeout = time.Second
	// httpBodyCap hard-caps the response body read for title extraction.
	httpBodyCap = 32 << 10
	// httpTitleMaxLen is the maximum title length in runes.
	httpTitleMaxLen = 80
)

// HTTPTitle GETs / on ip:port (HTTPS when https is true), reads at most 32
// KiB of the body, and extracts the first <title> element's text with HTML
// entities unescaped, whitespace trimmed, and a hard limit of 80 runes.
// A response without a title element is not an error: it returns
// (status, "", nil). The probe always uses a 1s client timeout; a longer
// deadline on ctx does not extend it. Redirects are never followed — the
// original response is returned as-is. TLS certificate verification is
// never disabled.
func HTTPTitle(ctx context.Context, ip netip.Addr, port int, https bool) (status int, title string, err error) {
	status, title, _, err = httpGet(ctx, ip, port, https)
	return status, title, err
}

// httpRootCAs is nil outside tests, which selects the system trust store.
var httpRootCAs *x509.CertPool

// leafCert is the certificate a server presented during the one handshake
// the title probe already makes. Trusted is false when verification failed:
// the dates are then a claim by an unauthenticated peer.
type leafCert struct {
	NotAfter time.Time
	Trusted  bool
}

// httpGet is HTTPTitle plus the presented leaf certificate. It makes the
// same single request under the same limits. When verification fails the
// certificate is read from the handshake error, never by retrying without
// verification.
func httpGet(ctx context.Context, ip netip.Addr, port int, https bool) (status int, title string, cert *leafCert, err error) {
	scheme := "http"
	transport := &http.Transport{}
	if https {
		scheme = "https"
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, RootCAs: httpRootCAs}
	}
	client := &http.Client{
		Timeout:   httpProbeTimeout,
		Transport: transport,
		// The original response's status/title is all the probe needs;
		// following redirects would be unintended egress.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()

	addr := netip.AddrPortFrom(ip, uint16(port))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+addr.String()+"/", nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Host = ip.String()

	resp, err := client.Do(req)
	if err != nil {
		var unverified *tls.CertificateVerificationError
		if errors.As(err, &unverified) {
			cert = leaf(unverified.UnverifiedCertificates, false)
		}
		return 0, "", cert, err
	}
	defer resp.Body.Close()
	if resp.TLS != nil {
		cert = leaf(resp.TLS.PeerCertificates, true)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, httpBodyCap))
	if err != nil {
		return resp.StatusCode, "", cert, err
	}
	return resp.StatusCode, extractTitle(string(body)), cert, nil
}

func leaf(chain []*x509.Certificate, trusted bool) *leafCert {
	if len(chain) == 0 {
		return nil
	}
	return &leafCert{NotAfter: chain[0].NotAfter, Trusted: trusted}
}

// ExtractTitle returns the first <title> element's text from body, with
// the same unescaping, trimming and 80-rune limit as the inventory probe.
func ExtractTitle(body string) string { return extractTitle(body) }

// extractTitle finds the first <title>...</title> element (case-insensitive,
// attributes tolerated), unescapes HTML entities, trims whitespace, and
// truncates to 80 runes.
func extractTitle(body string) string {
	// HTML tag names are ASCII case-insensitive. Unicode case folding can
	// change byte lengths, invalidating offsets into the original body.
	folded := []byte(body)
	for i, b := range folded {
		if b >= 'A' && b <= 'Z' {
			folded[i] = b + ('a' - 'A')
		}
	}
	lower := string(folded)
	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	gt := strings.IndexByte(lower[start:], '>')
	if gt < 0 {
		return ""
	}
	contentStart := start + gt + 1
	end := strings.Index(lower[contentStart:], "</title")
	if end < 0 {
		return ""
	}
	title := strings.TrimSpace(html.UnescapeString(body[contentStart : contentStart+end]))
	runes := []rune(title)
	if len(runes) > httpTitleMaxLen {
		title = string(runes[:httpTitleMaxLen])
	}
	return title
}

// detectWeb is the unexported seam behind web-service promotion (ruling
// R7): a plain-HTTP title probe, ok when the server answered with any HTTP
// status. ScanHost promotes open unknown ports to Web on ok and carries the
// extracted title.
func detectWeb(ctx context.Context, ip netip.Addr, port int) (title string, ok bool) {
	status, title, err := HTTPTitle(ctx, ip, port, false)
	if err != nil || status <= 0 {
		return "", false
	}
	return title, true
}

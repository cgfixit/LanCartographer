package probe

import (
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// httpTitleFixture starts an httptest server answering GET / with the given
// body, and returns the parsed loopback ip and port.
func httpTitleFixture(t *testing.T, body string) (netip.Addr, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		t.Fatalf("parse ip: %v", err)
	}
	return ip, port
}

func TestHTTPTitle(t *testing.T) {
	longTitle := strings.Repeat("a", 100)
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantTitle  string
	}{
		{
			name:       "normal title",
			body:       "<html><head><title>NAS Admin</title></head><body></body></html>",
			wantStatus: 200,
			wantTitle:  "NAS Admin",
		},
		{
			name:       "Unicode before tags preserves byte offsets",
			body:       strings.Repeat("Ⱥ", 100) + "<TITLE>NAS K Ⱥ Admin</TITLE>",
			wantStatus: 200,
			wantTitle:  "NAS K Ⱥ Admin",
		},
		{
			name:       "shorter lowercase encoding preserves title",
			body:       "K<TITLE>Kelvin</TITLE>",
			wantStatus: 200,
			wantTitle:  "Kelvin",
		},
		{
			name:       "invalid UTF-8 before title preserves offsets",
			body:       "\xff<TITLE>Device</TITLE>",
			wantStatus: 200,
			wantTitle:  "Device",
		},
		{
			name:       "long title truncated to 80 runes",
			body:       "<html><head><title>" + longTitle + "</title></head></html>",
			wantStatus: 200,
			wantTitle:  longTitle[:80],
		},
		{
			name:       "no title element",
			body:       "<html><head></head><body>hello</body></html>",
			wantStatus: 200,
			wantTitle:  "",
		},
		{
			name:       "garbage body",
			body:       "\x00\x01\x02 not html at all \xfe\xff",
			wantStatus: 200,
			wantTitle:  "",
		},
		{
			name:       "named entity unescaped",
			body:       "<title>Tom &amp; Jerry</title>",
			wantStatus: 200,
			wantTitle:  "Tom & Jerry",
		},
		{
			name:       "title tag with attributes",
			body:       `<title lang="en">X</title>`,
			wantStatus: 200,
			wantTitle:  "X",
		},
		{
			name:       "numeric entity unescaped",
			body:       "<title>&#65;BC</title>",
			wantStatus: 200,
			wantTitle:  "ABC",
		},
		{
			name:       "uppercase tags",
			body:       "<TITLE>Upper</TITLE>",
			wantStatus: 200,
			wantTitle:  "Upper",
		},
		{
			name:       "surrounding whitespace trimmed",
			body:       "<title>  padded  </title>",
			wantStatus: 200,
			wantTitle:  "padded",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip, port := httpTitleFixture(t, tc.body)
			status, title, err := HTTPTitle(context.Background(), ip, port, false)
			if err != nil {
				t.Fatalf("HTTPTitle: %v", err)
			}
			if status != tc.wantStatus {
				t.Errorf("status = %d, want %d", status, tc.wantStatus)
			}
			if title != tc.wantTitle {
				t.Errorf("title = %q, want %q", title, tc.wantTitle)
			}
		})
	}
}

func TestHTTPTitle_NoServer(t *testing.T) {
	ip := netip.MustParseAddr("127.0.0.1")
	port := closedPort(t)
	status, title, err := HTTPTitle(context.Background(), ip, port, false)
	if err == nil {
		t.Fatalf("expected error against closed port, got status=%d title=%q", status, title)
	}
}

func TestDetectWeb_ExtractsTitle(t *testing.T) {
	ip, port := httpTitleFixture(t, "<html><head><title>NAS Admin</title></head></html>")
	title, ok := detectWeb(context.Background(), ip, port)
	if !ok {
		t.Fatalf("detectWeb not ok against live HTTP fixture")
	}
	if title != "NAS Admin" {
		t.Fatalf("detectWeb title = %q, want %q", title, "NAS Admin")
	}
}

func TestHTTPTitle_DoesNotFollowRedirects(t *testing.T) {
	// Server B counts hits: a redirect-following probe would reach it.
	var hitsB atomic.Int32
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsB.Add(1)
		_, _ = w.Write([]byte("<title>redirect target</title>"))
	}))
	t.Cleanup(srvB.Close)

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvB.URL, http.StatusFound)
	}))
	t.Cleanup(srvA.Close)

	u, err := url.Parse(srvA.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		t.Fatalf("parse ip: %v", err)
	}

	status, _, err := HTTPTitle(context.Background(), ip, port, false)
	if err != nil {
		t.Fatalf("HTTPTitle: %v (a 302 response is not an error)", err)
	}
	if status != http.StatusFound {
		t.Errorf("status = %d, want 302 (the original response, not the redirect target)", status)
	}
	if got := hitsB.Load(); got != 0 {
		t.Errorf("redirect target received %d requests, want 0 (no unintended egress)", got)
	}
}

// tlsFixture starts an owned loopback HTTPS server with httptest's
// self-issued certificate, which the system trust store does not accept.
func tlsFixture(t *testing.T) (*httptest.Server, netip.Addr, int, *int32) {
	t.Helper()
	var requests int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		io.WriteString(w, "<title>tls fixture</title>")
	}))
	t.Cleanup(srv.Close)
	ap := netip.MustParseAddrPort(srv.Listener.Addr().String())
	return srv, ap.Addr(), int(ap.Port()), &requests
}

func TestHTTPGetReportsPresentedCertificate(t *testing.T) {
	srv, ip, port, requests := tlsFixture(t)
	want := srv.Certificate().NotAfter

	// Untrusted: the handshake fails, no request is sent and no title is
	// read, but the presented expiry is still reported and marked as such.
	status, title, cert, err := httpGet(context.Background(), ip, port, true)
	if err == nil || status != 0 || title != "" || atomic.LoadInt32(requests) != 0 {
		t.Fatalf("untrusted certificate was accepted: status=%d title=%q err=%v", status, title, err)
	}
	if cert == nil || cert.Trusted || !cert.NotAfter.Equal(want) {
		t.Fatalf("untrusted certificate not reported: %+v, want expiry %v", cert, want)
	}

	// Trusted: same single request, with the expiry marked as verified.
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	httpRootCAs = pool
	t.Cleanup(func() { httpRootCAs = nil })
	status, title, cert, err = httpGet(context.Background(), ip, port, true)
	if err != nil || status != http.StatusOK || title != "tls fixture" || atomic.LoadInt32(requests) != 1 {
		t.Fatalf("trusted probe: status=%d title=%q requests=%d err=%v", status, title, atomic.LoadInt32(requests), err)
	}
	if cert == nil || !cert.Trusted || !cert.NotAfter.Equal(want) {
		t.Fatalf("trusted certificate not reported: %+v", cert)
	}

	// Plain HTTP and a non-TLS answer on an HTTPS port report no certificate.
	pip, pport := httpTitleFixture(t, "<title>plain</title>")
	if _, _, cert, err := httpGet(context.Background(), pip, pport, false); err != nil || cert != nil {
		t.Fatalf("plain HTTP reported a certificate: %+v err=%v", cert, err)
	}
	if _, _, cert, err := httpGet(context.Background(), pip, pport, true); err == nil || cert != nil {
		t.Fatalf("non-TLS port reported a certificate: %+v err=%v", cert, err)
	}
	if leaf(nil, true) != nil {
		t.Fatal("an empty chain produced a certificate")
	}
}

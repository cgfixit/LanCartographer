package probe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
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

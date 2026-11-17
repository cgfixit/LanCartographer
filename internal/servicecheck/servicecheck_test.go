package servicecheck

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var loopback = netip.MustParseAddr("127.0.0.1")

func portOf(a net.Addr) uint16 { return uint16(a.(*net.TCPAddr).Port) }

func stage(t *testing.T, r Result, name string) Stage {
	t.Helper()
	for _, s := range r.Stages {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no %s stage in %+v", name, r.Stages)
	return Stage{}
}

// tlsFixture serves HTTPS for the names in the httptest certificate
// ("example.com" and 127.0.0.1) and records the SNI and Host it received.
func tlsFixture(t *testing.T) (*httptest.Server, *x509.CertPool, *atomic.Value) {
	t.Helper()
	seen := &atomic.Value{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.TLS.ServerName + "|" + r.Host)
		fmt.Fprint(w, "<title>Fixture</title>")
	}))
	srv.StartTLS()
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return srv, pool, seen
}

func TestCheckHTTPSVerifiesEnteredNameOnPinnedAddress(t *testing.T) {
	srv, pool, seen := tlsFixture(t)
	var dialed []string
	opts := Options{RootCAs: pool, Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}}
	target := Target{Host: "example.com", Addr: loopback, Port: portOf(srv.Listener.Addr()), Protocol: HTTPS}
	r, err := Check(context.Background(), target, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := stage(t, r, "TLS"); got.Outcome != Passed || !strings.Contains(got.Detail, "example.com") {
		t.Fatalf("TLS stage = %+v", got)
	}
	if got := stage(t, r, "HTTP"); got.Outcome != Passed || !strings.Contains(got.Detail, "200") || !strings.Contains(got.Detail, "Fixture") {
		t.Fatalf("HTTP stage = %+v", got)
	}
	want := fmt.Sprintf("example.com|example.com:%d", target.Port)
	if s, _ := seen.Load().(string); s != want {
		t.Fatalf("server saw SNI|Host %q, want %q", s, want)
	}
	if len(dialed) != 1 || dialed[0] != target.Endpoint() {
		t.Fatalf("dialed %v, want exactly [%s]", dialed, target.Endpoint())
	}
}

func TestCheckTLSFailuresNameTheCause(t *testing.T) {
	srv, pool, _ := tlsFixture(t)
	port := portOf(srv.Listener.Addr())
	cases := []struct {
		name string
		host string
		opts Options
		want string
	}{
		{"untrusted", "example.com", Options{}, "not trusted"},
		{"wrong name", "nas.example.lan", Options{RootCAs: pool}, "not valid for nas.example.lan"},
		{"expired", "example.com", Options{RootCAs: pool, Now: func() time.Time { return time.Now().AddDate(200, 0, 0) }}, "expired on"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := Check(context.Background(), Target{Host: tc.host, Addr: loopback, Port: port, Protocol: HTTPS}, tc.opts)
			if got := stage(t, r, "TLS"); got.Outcome != Failed || !strings.Contains(got.Detail, tc.want) {
				t.Fatalf("TLS stage = %+v, want %q", got, tc.want)
			}
			if got := stage(t, r, "HTTP"); got.Outcome != Skipped {
				t.Fatalf("HTTP ran after a TLS failure: %+v", got)
			}
		})
	}
}

func TestCheckDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Int32
	next := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed.Add(1) }))
	defer next.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, next.URL, http.StatusMovedPermanently)
	}))
	defer redirect.Close()
	r, _ := Check(context.Background(), Target{Host: "127.0.0.1", Addr: loopback, Port: portOf(redirect.Listener.Addr()), Protocol: HTTP}, Options{})
	if got := stage(t, r, "HTTP"); got.Outcome != Passed || !strings.Contains(got.Detail, "301") || !strings.Contains(got.Detail, "not followed") {
		t.Fatalf("HTTP stage = %+v", got)
	}
	if followed.Load() != 0 {
		t.Fatal("redirect target was contacted")
	}
}

func TestCheckSkipsInterimResponses(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 4096)
		c.Read(buf)
		c.Write([]byte("HTTP/1.1 100 Continue\r\n\r\nHTTP/1.1 503 Busy\r\nContent-Length: 0\r\n\r\n"))
	}()
	r, _ := Check(context.Background(), Target{Host: "127.0.0.1", Addr: loopback, Port: portOf(l.Addr()), Protocol: HTTP}, Options{})
	if got := stage(t, r, "HTTP"); got.Outcome != Warning || !strings.Contains(got.Detail, "503") {
		t.Fatalf("HTTP stage = %+v, want the final 503 as a warning", got)
	}
}

func TestCheckConnectOutcomes(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	free := portOf(l.Addr())
	l.Close()
	r, _ := Check(context.Background(), Target{Host: "127.0.0.1", Addr: loopback, Port: free, Protocol: HTTPS}, Options{})
	if got := stage(t, r, "Connect"); got.Outcome != Failed || !strings.Contains(got.Detail, "Refused") {
		t.Fatalf("refused connect = %+v", got)
	}
	if len(r.Stages) != 3 || r.Stages[1].Outcome != Skipped || r.Stages[2].Outcome != Skipped {
		t.Fatalf("later stages should be skipped: %+v", r.Stages)
	}

	hang := Options{Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	r, _ = Check(ctx, Target{Host: "127.0.0.1", Addr: loopback, Port: 1, Protocol: TCP}, hang)
	if !r.Cancelled || stage(t, r, "Connect").Detail != "Cancelled." {
		t.Fatalf("cancelled connect = %+v", r)
	}

	timeout := Options{Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, context.DeadlineExceeded
	}}
	r, _ = Check(context.Background(), Target{Host: "127.0.0.1", Addr: loopback, Port: 1, Protocol: TCP}, timeout)
	if got := stage(t, r, "Connect"); !strings.Contains(got.Detail, "No answer within 3 s") {
		t.Fatalf("timed-out connect = %+v", got)
	}
}

func TestCheckRejectsIncompleteTarget(t *testing.T) {
	dialed := false
	opts := Options{Dial: func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, errors.New("unexpected dial")
	}}
	for _, target := range []Target{
		{Host: "nas", Port: 80, Protocol: HTTP},
		{Host: "nas", Addr: loopback, Protocol: HTTP},
		{Host: "nas", Addr: loopback, Port: 80, Protocol: "FTP"},
	} {
		if _, err := Check(context.Background(), target, opts); err == nil {
			t.Fatalf("Check(%+v) accepted an incomplete target", target)
		}
	}
	if dialed {
		t.Fatal("an incomplete target was dialed")
	}
}

func TestHostHeader(t *testing.T) {
	v6 := netip.MustParseAddr("fe80::1%en0")
	for _, tc := range []struct {
		target Target
		want   string
	}{
		{Target{Host: "nas.lan", Addr: loopback, Port: 80, Protocol: HTTP}, "nas.lan"},
		{Target{Host: "nas.lan.", Addr: loopback, Port: 8443, Protocol: HTTPS}, "nas.lan:8443"},
		{Target{Host: "127.0.0.1", Addr: loopback, Port: 443, Protocol: HTTPS}, "127.0.0.1"},
		{Target{Host: v6.String(), Addr: v6, Port: 8080, Protocol: HTTP}, "[fe80::1]:8080"},
	} {
		if got := hostHeader(tc.target); got != tc.want {
			t.Errorf("hostHeader(%+v) = %q, want %q", tc.target, got, tc.want)
		}
	}
}

func TestParseHost(t *testing.T) {
	ifaces := func(name string) bool { return name == "en0" }
	for _, tc := range []struct {
		in, host string
		ok       bool
	}{
		{"  nas.home.arpa ", "nas.home.arpa", true},
		{"printer_2.local", "printer_2.local", true},
		{"192.168.1.20", "192.168.1.20", true},
		{"[2001:db8::5]", "2001:db8::5", true},
		{"::ffff:10.0.0.1", "10.0.0.1", true},
		{"fe80::1%en0", "fe80::1%en0", true},
		{"fe80::1", "", false},
		{"fe80::1%utun9", "", false},
		{"10.0.0.1%en0", "", false},
		{"::ffff:10.0.0.1%en0", "", false},
		{"fe80::1%en0\x00", "", false},
		{"https://nas.lan", "", false},
		{"nas.lan/admin", "", false},
		{"nas.lan:443", "", false},
		{"user@nas.lan", "", false},
		{"10.0.0.300", "", false},
		{"0.0.0.0", "", false},
		{"ff02::1", "", false},
		{"255.255.255.255", "", false},
		{"-nas.lan", "", false},
		{"nas\u202e.lan", "", false},
		{strings.Repeat("a", 64) + ".lan", "", false},
		{"", "", false},
	} {
		host, _, err := ParseHost(tc.in, ifaces)
		if (err == nil) != tc.ok || host != tc.host {
			t.Errorf("ParseHost(%q) = %q, %v; want %q ok=%v", tc.in, host, err, tc.host, tc.ok)
		}
	}
}

type fakeResolver struct {
	addrs []netip.Addr
	err   error
}

func (f fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return f.addrs, f.err
}

func TestResolveFiltersAndBounds(t *testing.T) {
	var many []netip.Addr
	many = append(many, netip.MustParseAddr("::ffff:192.168.1.20"), netip.MustParseAddr("192.168.1.20"),
		netip.MustParseAddr("fe80::9"), netip.MustParseAddr("0.0.0.0"))
	for i := 1; i <= 20; i++ {
		many = append(many, netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}))
	}
	addrs, st := Resolve(context.Background(), fakeResolver{addrs: many}, "nas.lan", nil)
	if st.Outcome != Passed || len(addrs) != MaxAddresses || addrs[0] != netip.MustParseAddr("192.168.1.20") {
		t.Fatalf("Resolve = %v, %+v", addrs, st)
	}
	if !strings.Contains(st.Detail, "omitted") {
		t.Fatalf("detail %q should report omitted answers", st.Detail)
	}

	_, st = Resolve(context.Background(), fakeResolver{err: &net.DNSError{Err: "no such host", Name: "nas.lan", IsNotFound: true}}, "nas.lan", nil)
	if st.Outcome != Failed || !strings.HasPrefix(st.Detail, "Name not found") {
		t.Fatalf("not-found stage = %+v", st)
	}
	_, st = Resolve(context.Background(), fakeResolver{addrs: []netip.Addr{netip.MustParseAddr("fe80::1")}}, "nas.lan", nil)
	if st.Outcome != Failed {
		t.Fatalf("an undialable-only answer should fail: %+v", st)
	}
}

func TestCleanNeutralizesUntrustedText(t *testing.T) {
	got := clean("a\x1b[2J\u202ebc\r\n\td\xff"+strings.Repeat("x", 300), 20)
	if strings.ContainsAny(got, "\x1b\u202e\r\n\t") || len([]rune(got)) != 20 || !strings.HasSuffix(got, "…") {
		t.Fatalf("clean = %q", got)
	}
}

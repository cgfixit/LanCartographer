package macdiag

import (
	"slices"
	"testing"
)

func TestRouteNumericDestination(t *testing.T) {
	for _, tc := range []struct {
		input  string
		family string
		target string
	}{
		{" 127.0.0.1 ", "-inet", "127.0.0.1"},
		{"::1", "-inet6", "::1"},
		{"::ffff:127.0.0.1", "-inet", "127.0.0.1"},
		{"fe80::1%lo0", "-inet6", "fe80::1%lo0"},
	} {
		got, err := readRouteArgs(tc.input)
		if err != nil || !slices.Equal(got, []string{"-n", "get", tc.family, tc.target}) {
			t.Fatalf("%q produced %v, %v", tc.input, got, err)
		}
	}
	for _, input := range []string{"", "localhost", "127.0.0.1:80", "127.0.0.0/8", "-ifscope",
		"fe80::1%missing-fixture-interface", "::ffff:127.0.0.1%lo0"} {
		if args, err := readRouteArgs(input); err == nil {
			t.Fatalf("invalid input %q produced %v", input, args)
		}
	}
}

func TestRouteNoRouteAndPlainText(t *testing.T) {
	if !noRouteReported([]byte("route: writing to routing socket: not in table\n")) ||
		noRouteReported([]byte("route: socket: Operation not permitted\n")) {
		t.Fatal("route errors conflated permissions and an absent route")
	}
	if got := configurationText([]byte("DNS configuration\n scoped\tresolver\x1b")); got != "DNS configuration\n scoped\tresolver�" {
		t.Fatalf("configuration display = %q", got)
	}
}

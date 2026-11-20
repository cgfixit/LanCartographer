package macdiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

const localReadLimit = 64 << 10

var errNoRoute = errors.New("macOS did not find a route for this destination")

func readRouteArgs(input string) ([]string, error) {
	ip, err := netip.ParseAddr(strings.TrimSpace(input))
	if err != nil {
		return nil, errors.New("Enter one numeric IPv4 or IPv6 address")
	}
	if ip.Is4In6() {
		if ip.Zone() != "" {
			return nil, errors.New("IPv4 addresses cannot use an IPv6 zone")
		}
		ip = ip.Unmap()
	}
	if zone := ip.Zone(); zone != "" {
		if _, err := net.InterfaceByName(zone); err != nil {
			return nil, errors.New("Use the name of a current interface for the IPv6 zone")
		}
	}
	family := "-inet"
	if ip.Is6() {
		family = "-inet6"
	}
	return []string{"-n", "get", family, ip.String()}, nil
}

func ReadRoute(ctx context.Context, input string) (string, error) {
	args, err := readRouteArgs(input)
	if err != nil {
		return "", err
	}
	text, err := readLocalConfiguration(ctx, "/sbin/route", args...)
	if err != nil {
		return "", err
	}
	if text == "" {
		return "", errors.New("macOS returned an empty route result")
	}
	return fmt.Sprintf("Route to %s\n\n%s", args[len(args)-1], text), nil
}

func ReadResolverConfiguration(ctx context.Context) (string, error) {
	return readLocalConfiguration(ctx, "/usr/sbin/scutil", "--dns")
}

func readLocalConfiguration(parent context.Context, path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	stdout, stderr, err := Run(ctx, localReadLimit, path, args...)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrOutputLimit) {
		return "", err
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("The system diagnostic helper is unavailable")
	}
	if err != nil {
		var exit *exec.ExitError
		if path == "/sbin/route" && errors.As(err, &exit) && noRouteReported(stderr) {
			return "", errNoRoute
		}
		return "", errors.New("Could not read the system configuration")
	}
	if len(strings.TrimSpace(string(stderr))) != 0 {
		return "", errors.New("The system diagnostic reported a warning and the result may be incomplete")
	}
	return configurationText(stdout), nil
}

func noRouteReported(stderr []byte) bool {
	for line := range strings.SplitSeq(string(stderr), "\n") {
		switch strings.TrimSpace(line) {
		case "route: writing to routing socket: not in table",
			"route: writing to routing socket: Network is unreachable",
			"route: writing to routing socket: No route to host":
			return true
		}
	}
	return false
}

func configurationText(output []byte) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || unicode.IsPrint(r) {
			return r
		}
		return '�'
	}, string(output)))
}

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"slices"

	"github.com/local/lancartographer/internal/model"
)

const (
	maxExpectationBytes  = 256 << 10
	maxExpectationChecks = 4096
)

type expectationsFile struct {
	Version int
	Hosts   []hostExpectation
}

type hostExpectation struct {
	IP     netip.Addr
	Open   []int
	Closed []int
	Absent bool
}

func (e *expectationsFile) UnmarshalJSON(data []byte) error {
	return decodeExpectationObject(data, map[string]any{"version": &e.Version, "hosts": &e.Hosts})
}

func (e *hostExpectation) UnmarshalJSON(data []byte) error {
	return decodeExpectationObject(data, map[string]any{
		"ip": &e.IP, "open": &e.Open, "closed": &e.Closed, "absent": &e.Absent,
	})
}

// Accept each named field exactly once. The default decoder accepts duplicate
// keys and may replace a declared check with a later value.
func decodeExpectationObject(data []byte, fields map[string]any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	first, err := d.Token()
	if err != nil {
		return err
	}
	if first != json.Delim('{') {
		return fmt.Errorf("expected a JSON object")
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return err
		}
		name := key.(string) // JSON object keys are strings
		dest, ok := fields[name]
		if !ok {
			return fmt.Errorf("unknown or duplicate field %q", name)
		}
		delete(fields, name)
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return err
		}
		if bytes.Equal(value, []byte("null")) {
			return fmt.Errorf("field %q must not be null", name)
		}
		if err := json.Unmarshal(value, dest); err != nil {
			return fmt.Errorf("field %q: %w", name, err)
		}
	}
	return nil // json.Unmarshal checked the object's closing delimiter
}

func loadExpectations(path, jsonPath string, targets []netip.Prefix, ports []int) ([]hostExpectation, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected a regular file")
	}
	if jsonPath != "" && jsonPath != "-" {
		if dst, err := os.Stat(jsonPath); err == nil && os.SameFile(info, dst) {
			return nil, fmt.Errorf("--expect and --json must name different files")
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxExpectationBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxExpectationBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxExpectationBytes)
	}

	var spec expectationsFile
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	if spec.Version != 1 {
		return nil, fmt.Errorf("expected version 1, got %d", spec.Version)
	}
	if len(spec.Hosts) == 0 {
		return nil, fmt.Errorf("at least one host expectation is required")
	}
	if err := validateExpectations(spec.Hosts, targets, ports); err != nil {
		return nil, err
	}
	return spec.Hosts, nil
}

func validateExpectations(hosts []hostExpectation, targets []netip.Prefix, ports []int) error {
	seenIPs := make(map[netip.Addr]bool)
	allowedPorts := make(map[int]bool)
	for _, port := range ports {
		allowedPorts[port] = true
	}
	count := 0
	for _, host := range hosts {
		ip := host.IP
		if !ip.Is4() || !slices.ContainsFunc(targets, func(target netip.Prefix) bool {
			// HostsIn excludes network and broadcast addresses except in /31 and /32.
			return target.Contains(ip) && (target.Bits() >= 31 ||
				(target.Contains(ip.Prev()) && target.Contains(ip.Next())))
		}) {
			return fmt.Errorf("host %q is outside the usable IPv4 scan range", ip)
		}
		if seenIPs[ip] {
			return fmt.Errorf("host %s is listed more than once", ip)
		}
		seenIPs[ip] = true

		portCount := len(host.Open) + len(host.Closed)
		if host.Absent == (portCount > 0) {
			return fmt.Errorf("host %s must expect either absence or port states", ip)
		}
		count += max(1, portCount)
		if count > maxExpectationChecks {
			return fmt.Errorf("file exceeds %d checks", maxExpectationChecks)
		}
		seenPorts := make(map[int]bool)
		for _, set := range [][]int{host.Open, host.Closed} {
			for _, port := range set {
				if !allowedPorts[port] {
					return fmt.Errorf("host %s port %d is outside the scan port list", ip, port)
				}
				if seenPorts[port] {
					return fmt.Errorf("host %s port %d is listed more than once", ip, port)
				}
				seenPorts[port] = true
			}
		}
	}
	return nil
}

func reportExpectations(w io.Writer, expected []hostExpectation, byIP map[netip.Addr]model.Host) bool {
	allMet := true
	for _, host := range expected {
		if host.Absent {
			_, found := byIP[host.IP]
			if found {
				fmt.Fprintf(w, "EXPECT %s absent: NOT MET (host observed)\n", host.IP)
				allMet = false
			} else {
				fmt.Fprintf(w, "EXPECT %s absent: MET (host not observed)\n", host.IP)
			}
			continue
		}

		states := make(map[int]string)
		for _, port := range byIP[host.IP].Ports {
			if prior, seen := states[port.Number]; seen && prior != port.State {
				states[port.Number] = "conflicting observations"
			} else {
				states[port.Number] = port.State
			}
		}
		for _, group := range []struct {
			ports []int
			want  string
		}{{host.Open, model.PortOpen}, {host.Closed, model.PortClosed}} {
			for _, port := range group.ports {
				got := states[port]
				result := "MET"
				if got != group.want {
					allMet = false
					result = "NOT MET"
				}
				if got == "" {
					got = "no port result"
				}
				fmt.Fprintf(w, "EXPECT %s:%d %s: %s (scan reported %s)\n", host.IP, port, group.want, result, got)
			}
		}
	}
	return allMet
}

// Package session stores explicit scan snapshots and compares recorded evidence.
// Reading, comparing and exporting sessions never runs a network probe.
package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/probe"
)

const (
	SchemaVersion = 1
	MaxBytes      = 8 << 20
	maxHosts      = 65536
	Complete      = "complete"
	Cancelled     = "cancelled"
	Failed        = "failed"
	Incomplete    = "incomplete"
)

// Profile records the effective inventory policy, independently of UI settings.
type Profile struct {
	Ports          []int  `json:"ports"`
	DialTimeoutMS  int64  `json:"dial_timeout_ms"`
	ProbeTimeoutMS int64  `json:"probe_timeout_ms"`
	Concurrency    int    `json:"concurrency"`
	Methods        string `json:"methods"`
}

// Source is a local interface snapshot, not a claim that sockets were bound to
// an interface. The OS chooses routes; DNS and route tables are not captured.
type Source struct {
	OS         string   `json:"os"`
	Arch       string   `json:"arch"`
	Interfaces []string `json:"interfaces"`
	Error      string   `json:"error,omitempty"`
}

type Session struct {
	Schema     int          `json:"schema"`
	AppVersion string       `json:"app_version"`
	Label      string       `json:"label"`
	Context    string       `json:"context"`
	Target     netip.Prefix `json:"target"`
	Profile    Profile      `json:"profile"`
	Source     Source       `json:"source"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	State      string       `json:"state"`
	Progress   float64      `json:"progress"`
	Hosts      records      `json:"hosts"`
}

// Limit record allocation while decoding, including arrays of empty objects.
type records []model.Host

func (r *records) UnmarshalJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	tok, err := d.Token()
	if err != nil || tok != json.Delim('[') {
		return errors.New("session hosts must be an array")
	}
	for d.More() {
		if len(*r) >= maxHosts {
			return fmt.Errorf("session exceeds %d host records", maxHosts)
		}
		var h model.Host
		if err := d.Decode(&h); err != nil {
			return err
		}
		*r = append(*r, h)
	}
	_, err = d.Token()
	return err
}

func New(target netip.Prefix, opts probe.Options, version string) Session {
	ports := slices.Clone(opts.Ports)
	slices.Sort(ports)
	ports = slices.Compact(ports)
	n := opts.Concurrency
	if n <= 0 {
		n = probe.DefaultConcurrency
	}
	n = min(n, probe.MaxConcurrency)
	methods := "inventory-v1:tcp+arp;names+cache;smb;http-1s-32KiB-verified-no-redirects"
	if runtime.GOOS == "darwin" {
		methods += ";icmp"
	}
	return Session{
		Schema: SchemaVersion, AppVersion: version, Target: target.Masked(),
		Profile: Profile{ports, opts.DialTimeout.Milliseconds(), opts.ProbeTimeout.Milliseconds(), n, methods},
		Source:  captureSource(), StartedAt: time.Now().UTC(), State: Incomplete,
		Hosts: records{},
	}
}

func captureSource() Source {
	s := Source{OS: runtime.GOOS, Arch: runtime.GOARCH, Interfaces: []string{}}
	ifs, err := net.Interfaces()
	if err != nil {
		s.Error = "interface enumeration failed"
		return s
	}
	for _, inf := range ifs {
		if inf.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := inf.Addrs()
		if err != nil {
			s.Error = "interface address enumeration failed"
			continue
		}
		for _, addr := range addrs {
			s.Interfaces = append(s.Interfaces, inf.Name+" "+addr.String())
		}
	}
	slices.Sort(s.Interfaces)
	return s
}

func (s Session) Clone() Session {
	s.Profile.Ports = slices.Clone(s.Profile.Ports)
	s.Source.Interfaces = slices.Clone(s.Source.Interfaces)
	s.Hosts = slices.Clone(s.Hosts)
	if s.Hosts == nil {
		s.Hosts = records{}
	}
	for i := range s.Hosts {
		s.Hosts[i].Ports = slices.Clone(s.Hosts[i].Ports)
		s.Hosts[i].Shares = slices.Clone(s.Hosts[i].Shares)
	}
	return s
}

func (s Session) Validate() error {
	if s.Schema != SchemaVersion {
		return fmt.Errorf("unsupported session schema %d (want %d)", s.Schema, SchemaVersion)
	}
	if strings.TrimSpace(s.Context) == "" || len(s.Context) > 160 || len(s.Label) > 160 {
		return errors.New("enter a site/context (1–160 bytes); label must be at most 160 bytes")
	}
	if s.AppVersion == "" || len(s.AppVersion) > 64 || s.Source.OS == "" || len(s.Source.OS) > 32 || len(s.Source.Arch) > 32 {
		return errors.New("invalid application or platform metadata")
	}
	if !s.Target.IsValid() || !s.Target.Addr().Is4() || s.Target != s.Target.Masked() || s.Target.Bits() < 12 {
		return errors.New("session target must be a masked IPv4 prefix, /12 or smaller")
	}
	if s.StartedAt.IsZero() || s.FinishedAt.IsZero() || s.FinishedAt.Before(s.StartedAt) {
		return errors.New("session needs valid start and finish times")
	}
	if math.IsNaN(s.Progress) || math.IsInf(s.Progress, 0) || s.Progress < 0 || s.Progress > 1 {
		return errors.New("invalid session progress")
	}
	switch s.State {
	case Complete:
		if s.Progress != 1 {
			return errors.New("complete session must have full progress")
		}
	case Cancelled, Failed, Incomplete:
	default:
		return errors.New("invalid session completion state")
	}
	if len(s.Source.Interfaces) > 256 || len(s.Source.Error) > 256 {
		return errors.New("too much source metadata")
	}
	for _, inf := range s.Source.Interfaces {
		if len(inf) > 512 {
			return errors.New("interface metadata too long")
		}
	}
	p := s.Profile
	if p.DialTimeoutMS <= 0 || p.ProbeTimeoutMS <= 0 || p.Concurrency < 1 || p.Concurrency > probe.MaxConcurrency || p.Methods == "" || len(p.Methods) > 256 {
		return errors.New("invalid probe profile")
	}
	ports := make(map[int]bool, len(p.Ports))
	for _, n := range p.Ports {
		if n < 1 || n > 65535 || ports[n] {
			return errors.New("invalid or duplicate profile port")
		}
		ports[n] = true
	}
	if len(ports) == 0 || len(s.Hosts) > maxHosts {
		return errors.New("empty probe profile or too many hosts")
	}
	seen := make(map[netip.Addr]bool, len(s.Hosts))
	for _, h := range s.Hosts {
		if !h.IP.IsValid() || !s.Target.Contains(h.IP) || seen[h.IP] {
			return errors.New("invalid, out-of-scope or duplicate host address")
		}
		seen[h.IP] = true
		if len(h.Shares) > 4096 {
			return errors.New("too many shares")
		}
		observed := map[int]bool{}
		for _, port := range h.Ports {
			if !ports[port.Number] || observed[port.Number] {
				return errors.New("unrequested or duplicate observed port")
			}
			observed[port.Number] = true
			if port.State != model.PortOpen && port.State != model.PortClosed && port.State != model.PortTimeout {
				return errors.New("invalid port observation")
			}
		}
	}
	return nil
}

func Read(r io.Reader) (Session, error) {
	var s Session
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return s, err
	}
	if len(data) > MaxBytes {
		return s, errors.New("session exceeds the 8 MiB file limit")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return s, fmt.Errorf("read session: %w", err)
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return s, errors.New("session contains trailing data")
	}
	return s, s.Validate()
}

func Save(path string, s Session) error {
	if err := s.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Clone(), "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, append(data, '\n'))
}

// WriteFile preserves the old file on failure and creates private files/dirs.
func WriteFile(path string, data []byte) error {
	if !filepath.IsAbs(path) {
		return errors.New("choose an absolute file path")
	}
	if len(data) > MaxBytes {
		return errors.New("output exceeds the 8 MiB file limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// Display keeps external text on one bounded line in labels and plain reports.
func Display(s string) string {
	r := []rune(s)
	if len(r) > 400 {
		r = append(r[:400], '…')
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return '�'
		}
		return r
	}, string(r))
}

package session

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/local/lancartographer/internal/model"
)

func fixture() Session {
	return Session{
		Schema: 1, AppVersion: "0.1.0", Label: "private-baseline", Context: "private-site",
		Target:     netip.MustParsePrefix("192.0.2.0/24"),
		Profile:    Profile{[]int{80, 443}, 400, 800, 128, "inventory-v1"},
		Source:     Source{OS: "darwin", Arch: "arm64", Interfaces: []string{"en0 192.0.2.50/24"}},
		StartedAt:  time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 11, 1, 12, 0, 5, 0, time.UTC), State: Complete, Progress: 1,
		Hosts: records{{IP: netip.MustParseAddr("192.0.2.10"), Alive: true, Name: "private-nas", MAC: "00:11:22:33:44:55",
			Ports:  []model.Port{{Number: 80, State: model.PortOpen, Title: "private-title"}, {Number: 443, State: model.PortClosed}},
			Shares: []model.Share{{Name: "private-share"}}}},
	}
}

func TestSessionSaveReadAndAtomicFailure(t *testing.T) {
	s := fixture()
	path := filepath.Join(t.TempDir(), "private", "baseline.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got.Context != s.Context || got.Hosts[0].Ports[0].Title != "private-title" {
		t.Fatal("lost session evidence")
	}
	// A recorded certificate round-trips through Save and Read.
	s.Hosts[0].Ports[0].CertExpires, s.Hosts[0].Ports[0].CertTrusted = "2027-12-25T18:34:12Z", true
	if err := Save(path, s); err != nil {
		t.Fatalf("rejected a recorded certificate: %v", err)
	}
	if data, err = os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if got, err = Read(bytes.NewReader(data)); err != nil || got.Hosts[0].Ports[0].CertExpires != "2027-12-25T18:34:12Z" || !got.Hosts[0].Ports[0].CertTrusted {
		t.Fatalf("certificate did not round-trip: %+v %v", got.Hosts[0].Ports[0], err)
	}
	s.Hosts[0].Ports[0].CertExpires, s.Hosts[0].Ports[0].CertTrusted = "", false
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		dir, _ := os.Stat(filepath.Dir(path))
		if info.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
			t.Fatal("session storage is not private")
		}
	}
	s.Schema = 99
	if err := Save(path, s); err == nil {
		t.Fatal("accepted unsupported schema")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("invalid save damaged the old file")
	}
	if err := WriteFile("relative.json", data); err == nil {
		t.Fatal("accepted relative output")
	}
	if err := WriteFile(path, make([]byte, MaxBytes+1)); err == nil {
		t.Fatal("accepted oversized output")
	}
	after, _ = os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("oversized save damaged old file")
	}
}

func TestSessionCloneOwnsNestedEvidence(t *testing.T) {
	original := fixture()
	copy := original.Clone()
	copy.Profile.Ports[0] = 22
	copy.Source.Interfaces[0] = "other"
	copy.Hosts[0].Ports[0].Title = "other"
	copy.Hosts[0].Shares[0].Name = "other"
	if original.Profile.Ports[0] != 80 || original.Source.Interfaces[0] == "other" || original.Hosts[0].Ports[0].Title != "private-title" || original.Hosts[0].Shares[0].Name != "private-share" {
		t.Fatal("snapshot shares mutable storage")
	}
}

func TestSessionReadRejectsUntrustedFiles(t *testing.T) {
	data, _ := json.Marshal(fixture())
	cases := map[string][]byte{
		"legacy inventory":   []byte(`[{"IP":"192.0.2.10"}]`),
		"trailing":           append(bytes.Clone(data), []byte(` {}`)...),
		"unknown field":      bytes.Replace(data, []byte(`"schema":1`), []byte(`"schema":1,"surprise":true`), 1),
		"unknown host field": bytes.Replace(data, []byte(`"Alive":true`), []byte(`"Alive":true,"surprise":true`), 1),
		"too large":          bytes.Repeat([]byte(" "), MaxBytes+1),
		"too many hosts":     []byte(`{"hosts":[` + strings.Repeat(`{},`, maxHosts) + `{}]}`),
		"empty":              nil,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Read(bytes.NewReader(data)); err == nil {
				t.Fatal("accepted invalid session")
			}
		})
	}
	for _, mutate := range []func(*Session){
		func(s *Session) { s.Hosts = append(s.Hosts, s.Hosts[0]) },
		func(s *Session) { s.Hosts[0].IP = netip.MustParseAddr("198.51.100.1") },
		func(s *Session) { s.Profile.Ports = []int{80, 80} },
		func(s *Session) { s.Hosts[0].Ports[0].Number = 22 },
		func(s *Session) { s.Progress = .5 },
		func(s *Session) { s.FinishedAt = s.StartedAt.Add(-time.Second) },
		// Certificate fields carry only what the scan records; the expiry is
		// shown even in a redacted report.
		func(s *Session) { s.Hosts[0].Ports[0].CertExpires = "private-nas admin:hunter2" },
		func(s *Session) { s.Hosts[0].Ports[0].CertExpires = "2026-12-01T00:00:00.5Z" },
		func(s *Session) { s.Hosts[0].Ports[0].CertExpires = "2026-12-01T01:00:00+01:00" },
		func(s *Session) { s.Hosts[0].Ports[1].CertExpires = "2026-12-01T00:00:00Z" },
		func(s *Session) { s.Hosts[0].Ports[0].CertTrusted = true },
	} {
		s := fixture()
		mutate(&s)
		data, _ := json.Marshal(s)
		if _, err := Read(bytes.NewReader(data)); err == nil {
			t.Fatal("accepted inconsistent session")
		}
	}
}

func TestSessionCompareCoverageAndObservations(t *testing.T) {
	b := fixture()
	a := b.Clone()
	a.Hosts[0].Name = "different-name"
	a.Hosts[0].MAC = "aa:bb:cc:dd:ee:ff"
	a.Hosts[0].Ports[0].State = model.PortTimeout
	c := Compare(b, a)
	if len(c.Reasons) != 0 || len(c.Changes) != 3 {
		t.Fatalf("unexpected comparison: %+v", c)
	}
	if c.Changes[2].After != model.PortTimeout {
		t.Fatal("timeout was relabelled")
	}
	a.Hosts = records{}
	c = Compare(b, a)
	if len(c.Changes) != 1 || c.Changes[0].After != "not observed" || c.Changes[0].Field != "Address observation" {
		t.Fatalf("absence inferred device/port state: %+v", c)
	}
	for name, mutate := range map[string]func(*Session){
		"cancelled":      func(s *Session) { s.State = Cancelled; s.Progress = .4 },
		"failed":         func(s *Session) { s.State = Failed },
		"scope":          func(s *Session) { s.Target = netip.MustParsePrefix("192.0.2.0/25") },
		"ports":          func(s *Session) { s.Profile.Ports = []int{80, 443, 8080} },
		"timeout":        func(s *Session) { s.Profile.ProbeTimeoutMS = 900 },
		"site":           func(s *Session) { s.Context = "another-client" },
		"source":         func(s *Session) { s.Source.Interfaces = []string{"vpn 198.51.100.1/32"} },
		"missing source": func(s *Session) { s.Source.Error = "unavailable" },
		"version":        func(s *Session) { s.AppVersion = "0.2.0" },
	} {
		t.Run(name, func(t *testing.T) {
			a := b.Clone()
			mutate(&a)
			c := Compare(b, a)
			if len(c.Reasons) == 0 || len(c.Changes) != 0 {
				t.Fatalf("incompatible data compared: %+v", c)
			}
		})
	}
	a = b.Clone()
	a.Profile.Ports = []int{443, 80}
	a.Hosts[0].Ports[0], a.Hosts[0].Ports[1] = a.Hosts[0].Ports[1], a.Hosts[0].Ports[0]
	if c := Compare(b, a); len(c.Reasons)+len(c.Changes) != 0 {
		t.Fatal("order created a false change")
	}
}

func TestSessionCompareAddressMove(t *testing.T) {
	moved := netip.MustParseAddr("192.0.2.41")
	move := func(mutate func(b, a *Session)) Comparison {
		b := fixture()
		a := b.Clone()
		a.Hosts[0].IP = moved
		if mutate != nil {
			mutate(&b, &a)
		}
		return Compare(b, a)
	}
	other := model.Host{IP: netip.MustParseAddr("192.0.2.77"), Alive: true, MAC: "00:11:22:33:44:66"}

	c := move(func(_, a *Session) { a.Hosts[0].Ports[1].State = model.PortOpen; a.Hosts[0].MAC = "00-11-22-33-44-55" })
	if len(c.Changes) != 2 || c.Changes[0] != (Change{IP: moved, Field: MovedField, Before: "192.0.2.10", After: "192.0.2.41", Address: true}) ||
		c.Changes[1].IP != moved || c.Changes[1].Field != "TCP 443 state" || c.Changes[1].After != model.PortOpen {
		t.Fatalf("move was not one paired change with its port difference: %+v", c.Changes)
	}
	for name, mutate := range map[string]func(b, a *Session){
		"locally administered":    func(b, a *Session) { b.Hosts[0].MAC, a.Hosts[0].MAC = "02:11:22:33:44:55", "02:11:22:33:44:55" },
		"group address":           func(b, a *Session) { b.Hosts[0].MAC, a.Hosts[0].MAC = "01:00:5e:00:00:fb", "01:00:5e:00:00:fb" },
		"empty":                   func(b, a *Session) { b.Hosts[0].MAC, a.Hosts[0].MAC = "", "" },
		"sentinel text":           func(b, a *Session) { b.Hosts[0].MAC, a.Hosts[0].MAC = "needs elevation", "needs elevation" },
		"zero":                    func(b, a *Session) { b.Hosts[0].MAC, a.Hosts[0].MAC = "00:00:00:00:00:00", "00:00:00:00:00:00" },
		"different MAC":           func(_, a *Session) { a.Hosts[0].MAC = "00:11:22:33:44:66" },
		"duplicate in Before":     func(b, _ *Session) { o := other; o.MAC = b.Hosts[0].MAC; b.Hosts = append(b.Hosts, o) },
		"duplicate in After":      func(_, a *Session) { o := other; o.MAC = a.Hosts[0].MAC; a.Hosts = append(a.Hosts, o) },
		"old address reused":      func(b, a *Session) { o := other; o.IP = b.Hosts[0].IP; a.Hosts = append(a.Hosts, o) },
		"new address seen before": func(b, a *Session) { o := other; o.IP = moved; b.Hosts = append(b.Hosts, o) },
	} {
		t.Run(name, func(t *testing.T) {
			for _, change := range move(mutate).Changes {
				if change.Address || change.Field == MovedField {
					t.Fatalf("weak evidence was paired: %+v", change)
				}
			}
		})
	}
	if c := move(nil); len(c.Changes) != 1 || !c.Changes[0].Address {
		t.Fatalf("unchanged moved device reported extra differences: %+v", c.Changes)
	}

	b := fixture()
	a := b.Clone()
	a.Hosts[0].IP = moved
	a.Hosts[0].Name = "secret-new-name"
	redacted, err := Report(b, a, []int{0, 1}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"192.0.2", "private", "secret", "00:11:22"} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("redacted move leaked %q:\n%s", forbidden, redacted)
		}
	}
	if !strings.Contains(redacted, "Address-1 | "+MovedField+"\n  Before: Address-2\n  After:  Address-1\n") || !strings.Contains(redacted, "it is not proof") {
		t.Fatalf("redacted move lost its aliases or caveat:\n%s", redacted)
	}
	full, err := Report(b, a, []int{1}, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(full, "192.0.2.10 ") || strings.Contains(full, "it is not proof") {
		t.Fatalf("unselected move leaked into the report:\n%s", full)
	}
}

func TestSessionCompareCertificate(t *testing.T) {
	b := fixture()
	b.Hosts[0].Ports[1] = model.Port{Number: 443, State: model.PortOpen, CertExpires: "2026-12-01T00:00:00Z"}
	a := b.Clone()
	if c := Compare(b, a); len(c.Reasons)+len(c.Changes) != 0 {
		t.Fatalf("identical certificates differed: %+v", c)
	}
	a.Hosts[0].Ports[1].CertExpires, a.Hosts[0].Ports[1].CertTrusted = "2027-12-01T00:00:00Z", true
	c := Compare(b, a)
	want := []Change{
		{IP: b.Hosts[0].IP, Field: "TCP 443 certificate expiry", Before: "2026-12-01T00:00:00Z", After: "2027-12-01T00:00:00Z"},
		{IP: b.Hosts[0].IP, Field: "TCP 443 certificate trust", Before: "not trusted", After: "trusted"},
	}
	if !slices.Equal(c.Changes, want) {
		t.Fatalf("renewal not reported as two differences: %+v", c.Changes)
	}
	a.Hosts[0].Ports[1].CertExpires, a.Hosts[0].Ports[1].CertTrusted = "", false
	if c := Compare(b, a); len(c.Changes) != 2 || c.Changes[1].After != "" {
		t.Fatalf("a certificate that is no longer presented was not reported: %+v", c.Changes)
	}
}

func TestSessionReportSelectionAndRedaction(t *testing.T) {
	b := fixture()
	a := b.Clone()
	a.Hosts[0].Name = "secret-new-name"
	a.Hosts[0].Ports[0].Title = "secret-new-title\x1b[31m\nhttps://secret.example/?token=secret"
	c := Compare(b, a)
	var title int
	for i, change := range c.Changes {
		if change.Field == "TCP 80 title" {
			title = i
		}
	}
	report, err := Report(b, a, []int{title}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"192.0.2", "private", "secret", "en0", "00:11:22", "Name\n"} {
		if strings.Contains(report, forbidden) {
			t.Fatalf("redacted report leaked %q", forbidden)
		}
	}
	if !strings.Contains(report, "Selected differences: 1 of 2") || !strings.Contains(report, "TCP 80 title") {
		t.Fatal("lost selection or evidence field")
	}
	full, err := Report(b, a, []int{title}, false, "Reviewed locally")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(full, "secret-new-name") || strings.Contains(full, "\x1b") || !strings.Contains(full, "secret-new-title") || !strings.Contains(full, "Reviewed locally") {
		t.Fatal("unselected data/control characters leaked or selected evidence lost")
	}
	for _, sel := range [][]int{nil, {-1}, {99}} {
		if _, err := Report(b, a, sel, true, ""); err == nil {
			t.Fatal("accepted invalid selection")
		}
	}
	a.State = Cancelled
	if _, err := Report(b, a, []int{title}, true, ""); err == nil {
		t.Fatal("exported incompatible comparison")
	}
}

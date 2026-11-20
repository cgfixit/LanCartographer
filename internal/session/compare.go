package session

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/local/lancartographer/internal/model"
)

type Change struct {
	IP        netip.Addr
	Field     string
	Before    string
	After     string
	Sensitive bool // discovered text is withheld in the default report
	Address   bool // Before and After are addresses, aliased like IP in a redacted report
}

// MovedField labels the one change that pairs two addresses by MAC.
const MovedField = "Address (matched by MAC)"

type Comparison struct {
	Reasons []string
	Changes []Change
}

func Compare(before, after Session) Comparison {
	c := Comparison{}
	for _, side := range []struct {
		name string
		s    Session
	}{{"Before", before}, {"After", after}} {
		if err := side.s.Validate(); err != nil {
			c.Reasons = append(c.Reasons, side.name+": "+err.Error())
		}
		if side.s.State != Complete {
			c.Reasons = append(c.Reasons, side.name+" scan is incomplete; absence cannot establish a change.")
		}
		if side.s.Source.Error != "" || len(side.s.Source.Interfaces) == 0 {
			c.Reasons = append(c.Reasons, side.name+" source interface snapshot is unavailable.")
		}
	}
	if before.Context != after.Context {
		c.Reasons = append(c.Reasons, "Site/context labels differ.")
	}
	if before.Target != after.Target {
		c.Reasons = append(c.Reasons, "Requested address scopes differ.")
	}
	bp, ap := before.Profile, after.Profile
	bp.Ports, ap.Ports = slices.Clone(bp.Ports), slices.Clone(ap.Ports)
	slices.Sort(bp.Ports)
	slices.Sort(ap.Ports)
	if !reflect.DeepEqual(bp, ap) || before.AppVersion != after.AppVersion {
		c.Reasons = append(c.Reasons, "Probe profiles or application versions differ.")
	}
	bs, as := before.Source, after.Source
	bs.Interfaces, as.Interfaces = slices.Clone(bs.Interfaces), slices.Clone(as.Interfaces)
	slices.Sort(bs.Interfaces)
	slices.Sort(as.Interfaces)
	if !reflect.DeepEqual(bs, as) {
		c.Reasons = append(c.Reasons, "Recorded source interfaces or platform differ (routes are OS-selected).")
	}
	if after.StartedAt.Before(before.StartedAt) {
		c.Reasons = append(c.Reasons, "After session predates Before; swap the sessions.")
	}
	if len(c.Reasons) != 0 {
		return c
	}
	bHosts, aHosts := map[netip.Addr]model.Host{}, map[netip.Addr]model.Host{}
	all := map[netip.Addr]bool{}
	for _, h := range before.Hosts {
		bHosts[h.IP], all[h.IP] = h, true
	}
	for _, h := range after.Hosts {
		aHosts[h.IP], all[h.IP] = h, true
	}
	// A device whose address changed is paired by MAC so it reads as one
	// change and its ports are still compared. Only a clean move pairs: the
	// MAC is universally administered, appears once in each session, and
	// neither address was observed in the other session.
	bMACs, aMACs := stableMACs(before.Hosts), stableMACs(after.Hosts)
	movedTo, movedFrom := map[netip.Addr]netip.Addr{}, map[netip.Addr]netip.Addr{}
	for mac, from := range bMACs {
		to, ok := aMACs[mac]
		if _, reused := aHosts[from]; !ok || reused {
			continue
		}
		if _, known := bHosts[to]; !known {
			movedTo[from], movedFrom[to] = to, from
		}
	}
	ips := make([]netip.Addr, 0, len(all))
	for ip := range all {
		ips = append(ips, ip)
	}
	slices.SortFunc(ips, func(a, b netip.Addr) int { return a.Compare(b) })
	for _, ip := range ips {
		if _, moved := movedTo[ip]; moved {
			continue // reported once, at the address observed in After
		}
		b, bok := bHosts[ip]
		a, aok := aHosts[ip]
		add := func(field, old, new string, sensitive bool) {
			if old != new {
				c.Changes = append(c.Changes, Change{IP: ip, Field: field, Before: old, After: new, Sensitive: sensitive})
			}
		}
		if from, moved := movedFrom[ip]; moved {
			b, bok = bHosts[from], true
			b.MAC = a.MAC // the pair matched on it; a spelling difference is not a change
			c.Changes = append(c.Changes, Change{IP: ip, Field: MovedField, Before: from.String(), After: ip.String(), Address: true})
		}
		if !bok || !aok {
			observed := func(ok bool) string {
				if ok {
					return "observed"
				}
				return "not observed"
			}
			add("Address observation", observed(bok), observed(aok), false)
			continue // a missing address supplies no evidence about individual ports
		}
		add("Name", b.Name, a.Name, true)
		add("MAC", b.MAC, a.MAC, true)
		add("Vendor", b.Vendor, a.Vendor, true)
		add("Discovery method", b.FoundVia, a.FoundVia, true)
		bm, am := map[int]model.Port{}, map[int]model.Port{}
		for _, p := range b.Ports {
			bm[p.Number] = p
		}
		for _, p := range a.Ports {
			am[p.Number] = p
		}
		for _, n := range bp.Ports {
			b, a := bm[n], am[n]
			add(fmt.Sprintf("TCP %d state", n), b.State, a.State, false)
			add(fmt.Sprintf("TCP %d service", n), b.Service, a.Service, true)
			add(fmt.Sprintf("TCP %d title", n), b.Title, a.Title, true)
			add(fmt.Sprintf("TCP %d certificate expiry", n), b.CertExpires, a.CertExpires, false)
			add(fmt.Sprintf("TCP %d certificate trust", n), certTrust(b), certTrust(a), false)
		}
		add("SMB shares", shares(b.Shares), shares(a.Shares), true)
	}
	return c
}

// stableMACs maps each MAC that can identify a device within one session to
// its address. Locally administered addresses are left out because privacy
// features rotate them, group addresses because they name no single device,
// and a MAC seen on several addresses because it is usually a router.
func stableMACs(hosts []model.Host) map[string]netip.Addr {
	out, seen := map[string]netip.Addr{}, map[string]int{}
	for _, h := range hosts {
		hw, err := net.ParseMAC(h.MAC)
		if err != nil || len(hw) != 6 || hw[0]&3 != 0 || hw.String() == "00:00:00:00:00:00" {
			continue
		}
		key := hw.String()
		seen[key]++
		out[key] = h.IP
	}
	for key, n := range seen {
		if n > 1 {
			delete(out, key)
		}
	}
	return out
}

func certTrust(p model.Port) string {
	switch {
	case p.CertExpires == "":
		return ""
	case p.CertTrusted:
		return "trusted"
	}
	return "not trusted"
}

func shares(shares []model.Share) string {
	copy := slices.Clone(shares)
	sort.Slice(copy, func(i, j int) bool {
		return strings.Join([]string{copy[i].Name, copy[i].Type, copy[i].Remark, copy[i].UNC}, "\x00") <
			strings.Join([]string{copy[j].Name, copy[j].Type, copy[j].Remark, copy[j].UNC}, "\x00")
	})
	var rows []string
	for _, s := range copy {
		rows = append(rows, fmt.Sprintf("%q (%q) %q %q", s.Name, s.Type, s.UNC, s.Remark))
	}
	return strings.Join(rows, "; ")
}

func Value(value string) string {
	if value == "" {
		return "not recorded"
	}
	return Display(value)
}

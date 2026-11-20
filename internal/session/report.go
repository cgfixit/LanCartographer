package session

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Report contains only selected differences and their coverage context. Plain
// text keeps discovered values inert, with no remote assets or active markup.
func Report(before, after Session, selected []int, redact bool, notes string) (string, error) {
	c := Compare(before, after)
	if len(c.Reasons) > 0 {
		return "", errors.New("sessions are not comparable: " + strings.Join(c.Reasons, " "))
	}
	if len(selected) == 0 {
		return "", errors.New("select at least one difference")
	}
	if len(notes) > 2000 {
		return "", errors.New("notes must be at most 2000 bytes")
	}
	selected = slices.Clone(selected)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	for _, i := range selected {
		if i < 0 || i >= len(c.Changes) {
			return "", errors.New("invalid evidence selection")
		}
	}
	var b strings.Builder
	fmt.Fprintln(&b, "LanCartographer — selected comparison evidence")
	fmt.Fprintln(&b, "Both scans completed with matching scope, profile and recorded source context.")
	fmt.Fprintln(&b, "Observations are address-based; they do not establish permanent device identity.")
	if slices.ContainsFunc(selected, func(i int) bool { return c.Changes[i].Address }) {
		fmt.Fprintln(&b, "\"Matched by MAC\" pairs two addresses that reported one hardware address; it is not proof.")
	}
	fmt.Fprintln(&b, "Not observed does not mean offline. Missing enrichment means not recorded.")
	fmt.Fprintln(&b, "Names may come from cache. Routes/DNS are not captured; routing is OS-selected.")
	if redact {
		fmt.Fprintln(&b, "Identifiers and discovered text withheld; report-local aliases are not anonymization.")
	} else {
		fmt.Fprintln(&b, "Identifiers and selected discovered text included. Review before sharing.")
	}
	fmt.Fprintf(&b, "\nSelected differences: %d of %d\n", len(selected), len(c.Changes))
	for _, side := range []struct {
		name string
		s    Session
	}{{"Before", before}, {"After", after}} {
		s := side.s
		fmt.Fprintf(&b, "%s: %s — %s; %s; %.0f%%; %d address records\n", side.name,
			s.StartedAt.Format(time.RFC3339), s.FinishedAt.Format(time.RFC3339), s.State, s.Progress*100, len(s.Hosts))
		if !redact {
			fmt.Fprintf(&b, "  Label: %s; context: %s; scope: %s; app: %s; %s/%s\n",
				Display(s.Label), Display(s.Context), s.Target, Display(s.AppVersion), Display(s.Source.OS), Display(s.Source.Arch))
			for _, inf := range s.Source.Interfaces {
				fmt.Fprintf(&b, "  Interface: %s\n", Display(inf))
			}
		} else {
			fmt.Fprintln(&b, "  Context, scope and source identifiers withheld (matching in both sessions).")
		}
	}
	ports := slices.Clone(before.Profile.Ports)
	slices.Sort(ports)
	fmt.Fprintf(&b, "Profile: TCP ports %v; discovery dial %dms; probe dial %dms; concurrency %d\n",
		ports, before.Profile.DialTimeoutMS, before.Profile.ProbeTimeoutMS, before.Profile.Concurrency)
	if !redact {
		fmt.Fprintf(&b, "Methods: %s\n", Display(before.Profile.Methods))
	}
	aliases := map[string]string{}
	alias := func(key, prefix string) string {
		if a, ok := aliases[key]; ok {
			return a
		}
		a := fmt.Sprintf("%s-%d", prefix, len(aliases)+1)
		aliases[key] = a
		return a
	}
	for _, i := range selected {
		d := c.Changes[i]
		ip := d.IP.String()
		old, new := Value(d.Before), Value(d.After)
		if redact {
			ip = alias("ip:"+ip, "Address")
			if d.Address {
				old, new = alias("ip:"+d.Before, "Address"), alias("ip:"+d.After, "Address")
			}
			if d.Sensitive {
				if d.Before != "" {
					old = alias("text:"+d.Before, "Value")
				}
				if d.After != "" {
					new = alias("text:"+d.After, "Value")
				}
			}
		}
		fmt.Fprintf(&b, "\n%s | %s\n  Before: %s\n  After:  %s\n", ip, d.Field, old, new)
	}
	if notes != "" {
		fmt.Fprintln(&b, "\nOperator notes (included as entered; review for identifiers):")
		for _, line := range strings.Split(notes, "\n") {
			fmt.Fprintln(&b, "  "+Display(line))
		}
	}
	if b.Len() > MaxBytes {
		return "", errors.New("report exceeds the 8 MiB limit")
	}
	return b.String(), nil
}

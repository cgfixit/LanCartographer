package gui

import (
	"sort"
	"sync"

	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/scan"
)

// hostStore merges engine events into one record per IP (R10/R11): port
// and share events append to the existing record (creating a stub when
// the host is unknown), while the canonical EventHostFound REPLACES the
// record with the fully enriched one. Widget code reads the store through
// snapshot(), which is also what sorting operates on, so the GUI never
// holds model.Host pointers that events mutate.
type hostStore struct {
	mu    sync.Mutex
	hosts map[string]*model.Host
	order []string // insertion order of first appearance, for stable listing
}

func newHostStore() *hostStore {
	return &hostStore{hosts: map[string]*model.Host{}}
}

// apply folds one engine event into the store. It returns the affected
// host's IP string ("" when the event carries no host, e.g. progress).
func (s *hostStore) apply(ev scan.Event) string {
	switch ev.Kind {
	case scan.EventHostFound, scan.EventPortFound, scan.EventShareFound:
	default:
		return ""
	}
	ip := ev.Host.IP.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.hosts[ip]
	if !ok {
		h = &model.Host{IP: ev.Host.IP}
		s.hosts[ip] = h
		s.order = append(s.order, ip)
	}
	if ev.Kind == scan.EventHostFound {
		// Canonical record wins wholesale — but never downgrade Alive.
		replaced := ev.Host
		if h.Alive {
			replaced.Alive = true
		}
		*h = replaced
		return ip
	}
	h.Ports = append(h.Ports, ev.Host.Ports...)
	h.Shares = append(h.Shares, ev.Host.Shares...)
	if len(ev.Host.Ports)+len(ev.Host.Shares) > 0 {
		h.Alive = true // a live port/share observation implies liveness
	}
	return ip
}

// set replaces the inventory wholesale (Load Favorites path) — no merge.
func (s *hostStore) set(hosts []model.Host) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts = map[string]*model.Host{}
	s.order = s.order[:0]
	for i := range hosts {
		h := hosts[i]
		key := h.IP.String()
		if _, dup := s.hosts[key]; dup {
			continue
		}
		s.hosts[key] = &h
		s.order = append(s.order, key)
	}
}

// snapshot returns a copy of every host, sorted by IP. Callers own the
// returned slice and its records.
func (s *hostStore) snapshot() []model.Host {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Host, 0, len(s.hosts))
	for _, ip := range s.order {
		out = append(out, *s.hosts[ip])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP.Compare(out[j].IP) < 0 })
	return out
}

// get returns a copy of one host's record, ok=false when unknown.
func (s *hostStore) get(ip string) (model.Host, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.hosts[ip]
	if !ok {
		return model.Host{}, false
	}
	return *h, true
}

// len reports how many host records the store holds.
func (s *hostStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.hosts)
}

// counts returns (hosts, open ports, shares) for the status bar.
func (s *hostStore) counts() (hosts, openPorts, shares int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.hosts {
		hosts++
		for _, p := range h.Ports {
			if p.State == model.PortOpen {
				openPorts++
			}
		}
		for _, share := range h.Shares {
			if !isDenialShare(share) {
				shares++
			}
		}
	}
	return hosts, openPorts, shares
}

//go:build cgo

package gui

import (
	"net/netip"
	"testing"

	"github.com/local/lancartographer/internal/macdiag"
	"github.com/local/lancartographer/internal/model"
	"github.com/local/lancartographer/internal/session"
)

func TestConnectionsInventoryLink(t *testing.T) {
	address := netip.MustParseAddr("127.0.0.1")
	current := &session.Session{State: session.Complete, Target: netip.MustParsePrefix("127.0.0.1/32")}
	u := &ui{currentSession: current, sorted: []model.Host{{IP: address}}}
	v := &connectionsView{owner: u, selected: &macdiag.Connection{HasPeer: true, Peer: macdiag.Endpoint{IP: address}}}
	if row := v.inventoryRow(); row != 0 {
		t.Fatalf("exact IPv4 peer match = %d", row)
	}
	for _, ip := range []string{"127.0.0.2", "::ffff:127.0.0.1", "::1"} {
		v.selected.Peer.IP = netip.MustParseAddr(ip)
		if row := v.inventoryRow(); row >= 0 {
			t.Fatalf("unmatched peer %s linked to inventory row %d", ip, row)
		}
	}
	v.selected.Peer.IP = address
	u.currentSession = nil
	if row := v.inventoryRow(); row >= 0 {
		t.Fatalf("favorites linked as a completed scan at row %d", row)
	}
	u.currentSession = current
	current.State = session.Cancelled
	if row := v.inventoryRow(); row >= 0 {
		t.Fatalf("cancelled scan linked to inventory row %d", row)
	}
}

//go:build cgo && !darwin

package gui

import (
	"net/netip"

	"fyne.io/fyne/v2"
)

type connectionsUI struct{}

func (*connectionsUI) stopAndWait() {}

func (*ui) connectionsMenu() []*fyne.MenuItem { return nil }

func (*ui) connectionHostMenu(netip.Addr) []*fyne.MenuItem { return nil }

func (*ui) refreshConnectionInventory() {}

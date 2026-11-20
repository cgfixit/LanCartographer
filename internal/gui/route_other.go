//go:build cgo && !darwin

package gui

import "fyne.io/fyne/v2"

type routeUI struct{}

func (*routeUI) stopAndWait() {}

func (*ui) routeMenu() []*fyne.MenuItem { return nil }

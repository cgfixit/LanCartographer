//go:build !cgo

package gui

import "errors"

// Run is unreachable in a real build: the Fyne GUI requires cgo (OpenGL
// driver). This stub exists so non-cgo builds (cross-compile smoke checks
// for darwin/linux from a Windows CI job) still compile the package graph.
// A user who actually runs a no-cgo binary gets a clear failure instead of
// a link-time error.
func Run() error {
	return errors.New("gui: this build has no CGO support; the Fyne GUI requires cgo — use the CLI flag or a CGO-enabled build")
}

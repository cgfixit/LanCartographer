// LanCartographer entry point. With --cli as the first argument it runs
// the table CLI and exits with its code; anything else opens the Fyne GUI.
package main

import (
	"fmt"
	"os"

	"github.com/local/lancartographer/internal/cli"
	"github.com/local/lancartographer/internal/gui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--cli" {
		os.Exit(cli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	if err := gui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lancartographer:", err)
		os.Exit(1)
	}
}

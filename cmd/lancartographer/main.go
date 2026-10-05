// LanCartographer entry point. It runs the table CLI when the executable
// name marks it as the CLI build (e.g. lancartographer-cli.exe) or when
// --cli is passed as the first argument; anything else opens the Fyne GUI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/local/lancartographer/internal/cli"
	"github.com/local/lancartographer/internal/gui"
)

// wantsCLI decides whether this invocation runs the CLI. exeBase is the
// executable's base name (e.g. from os.Args[0]); args is os.Args[1:]. It
// reports the CLI mode when the executable name contains "cli"
// (case-insensitive, e.g. lancartographer-cli.exe) or when args begins with
// "--cli", and returns the args with that token stripped when present.
func wantsCLI(exeBase string, args []string) (bool, []string) {
	cli := strings.Contains(strings.ToLower(exeBase), "cli")
	if len(args) > 0 && args[0] == "--cli" {
		cli = true
		args = args[1:]
	}
	return cli, args
}

func main() {
	cliMode, args := wantsCLI(filepath.Base(os.Args[0]), os.Args[1:])
	if cliMode {
		os.Exit(cli.Run(args, os.Stdout, os.Stderr))
	}
	if err := gui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lancartographer:", err)
		os.Exit(1)
	}
}

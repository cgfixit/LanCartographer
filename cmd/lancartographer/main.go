// LanCartographer entry point. With --cli as the first argument it runs
// the table CLI and exits with its code; anything else reports that the
// GUI is not yet implemented (Task 19) and exits 1.
package main

import (
	"fmt"
	"os"

	"github.com/local/lancartographer/internal/cli"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--cli" {
		os.Exit(cli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	fmt.Fprintln(os.Stderr, "LanCartographer GUI is not yet implemented (arrives in Task 19). Use --cli.")
	os.Exit(1)
}

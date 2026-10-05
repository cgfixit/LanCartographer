//go:build darwin

package winutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsElevated reports whether the process is running as root.
func IsElevated() bool {
	return os.Geteuid() == 0
}

// RelaunchElevated starts a detached copy of the current executable with
// administrator privileges via AppleScript, which prompts the operator for
// credentials. It returns as soon as osascript reports back (an error here
// means the prompt was declined or failed); the caller is expected to exit
// so only the elevated instance keeps running. The executable path is
// single-quote escaped so no path content is ever interpreted by the shell,
// and osascript is invoked with an argv list, never a shell string.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("winutil: cannot locate own executable: %w", err)
	}
	// POSIX single-quote escaping: only ' is special inside single quotes,
	// and it is closed out as '\''. No AppleScript-level escaping is needed
	// because ' is literal inside an AppleScript double-quoted string.
	quoted := "'" + strings.ReplaceAll(exe, "'", `'\''`) + "'"
	script := `do shell script "` + quoted + ` > /dev/null 2>&1 &" with administrator privileges`
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("winutil: elevation request declined or failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

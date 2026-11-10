package probe

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultPortsCSV is the default inventory scan profile shared by the GUI and CLI.
const DefaultPortsCSV = "21,22,23,80,135,139,443,445,515,631,3389,5357,8080,8443,9100"

// ParsePorts validates a comma-separated port list in input order.
// Empty fields are invalid and repeated ports are preserved.
func ParsePorts(s string) ([]int, error) {
	var ports []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid port %q", part)
		}
		ports = append(ports, n)
	}
	return ports, nil
}

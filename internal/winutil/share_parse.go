package winutil

import (
	"regexp"
	"strings"
)

// Share type codes emitted by parseSMBUtilView. They deliberately match the
// Windows STYPE_* values that internal/probe filters on, so share handling
// downstream is identical on both platforms.
const (
	smbTypeDisk    = 0          // STYPE_DISKTREE
	smbTypePrint   = 1          // STYPE_PRINTQ
	smbTypeDevice  = 2          // STYPE_DEVICE (filtered out downstream)
	smbTypeIPC     = 3          // STYPE_IPC
	smbTypeSpecial = 0x80000000 // STYPE_SPECIAL: hidden admin share
)

// smbutil prints the type left-aligned in an eight-byte field.
var smbViewTypeField = regexp.MustCompile(`(?i)Disk {4}|Printer |IPC {5}|Pipe {4}|Comm {4}|Device {2}|Unknown `)

// parseSMBUtilView reads the byte columns printed by macOS smbutil view.
// A share ending in '$' carries STYPE_SPECIAL for the inventory filter.
func parseSMBUtilView(output string) []ShareInfo1 {
	var out []ShareInfo1
	typeColumn := 0
	for line := range strings.Lines(output) {
		line = strings.TrimRight(line, "\r\n")
		if typeColumn == 0 {
			header := strings.Fields(line)
			if len(header) == 3 && header[0] == "Share" && header[1] == "Type" &&
				(header[2] == "Comments" || header[2] == "Comment") {
				typeColumn = strings.Index(line, "Type")
			}
			continue
		}
		if len(line) <= typeColumn {
			continue
		}
		tail := line[typeColumn:]
		fields := smbViewTypeField.FindAllStringIndex(tail, 2)
		if len(fields) == 0 {
			continue
		}
		field := fields[0]
		// When a name fills its column, smbutil appends the type without
		// a delimiter. Skip rows with more than one possible split.
		if len(fields) > 1 && (field[0] != 0 || line[typeColumn-1] != ' ') {
			continue
		}
		name := strings.TrimRight(line[:typeColumn+field[0]], " ")
		if name == "" {
			continue
		}
		typ := uint32(smbTypeDevice)
		switch strings.ToLower(strings.TrimSpace(tail[field[0]:field[1]])) {
		case "disk":
			typ = smbTypeDisk
		case "printer":
			typ = smbTypePrint
		case "ipc":
			typ = smbTypeIPC
		}
		if strings.HasSuffix(name, "$") {
			typ |= smbTypeSpecial
		}
		out = append(out, ShareInfo1{
			Name:   name,
			Type:   typ,
			Remark: strings.TrimSpace(tail[field[1]:]),
		})
	}
	return out
}

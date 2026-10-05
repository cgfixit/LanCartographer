package winutil

import "strings"

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

// parseSMBUtilView parses the table printed by macOS `smbutil view //host`:
//
//	Share             Type    Comment
//	-------------------------------
//	backups           Disk    nightly backups
//	IPC$              IPC
//	print$            Printer
//
// Names ending in '$' carry the STYPE_SPECIAL bit (mirroring NetShareEnum,
// which marks IPC$/print$/ADMIN$ hidden) so the downstream filter treats
// them identically on both platforms. Unknown Type values map to
// smbTypeDevice. Header, separator, and blank lines are skipped. It is pure
// (no I/O) so it is table-tested on any OS.
func parseSMBUtilView(output string) []ShareInfo1 {
	var out []ShareInfo1
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "Share" && fields[1] == "Type" {
			continue // header row
		}
		if strings.TrimLeft(fields[0], "-") == "" {
			continue // separator row
		}
		var typ uint32
		switch strings.ToLower(fields[1]) {
		case "disk":
			typ = smbTypeDisk
		case "printer":
			typ = smbTypePrint
		case "ipc":
			typ = smbTypeIPC
		default:
			typ = smbTypeDevice
		}
		if strings.HasSuffix(fields[0], "$") {
			typ |= smbTypeSpecial
		}
		out = append(out, ShareInfo1{
			Name:   fields[0],
			Type:   typ,
			Remark: strings.Join(fields[2:], " "),
		})
	}
	return out
}

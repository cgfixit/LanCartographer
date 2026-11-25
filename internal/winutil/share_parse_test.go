package winutil

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseSMBUtilView(t *testing.T) {
	row := func(name, typ, remark string) string {
		// smbutil's printf measures field width in bytes, including UTF-8.
		return name + strings.Repeat(" ", max(0, 48-len(name))) + fmt.Sprintf("%-8s%s\n", typ, remark)
	}
	header := row("Share", "Type", "Comments") + "-------------------------------\n"
	longName := "Archive team international engineering planning 2026"
	fullWidth := longName[:47] + "x"
	longHidden := longName + "$"
	ambiguous := longName + " Disk    private$"
	tests := []struct {
		name   string
		output string
		want   []ShareInfo1
	}{
		{
			name: "smbutil view table",
			output: header +
				row("Team Files", "Disk", "Quarterly  planning") +
				row("Team  Disk Files", "Disk", "double space in the name") +
				row("Résumé archives", "Disk", "UTF-8 name") +
				row("media", "Disk", "Disk    in the comment") +
				row("Office Printer", "Printer", "shared printer") +
				row("IPC$", "IPC", "") +
				row("Hidden Files$", "Disk", "") +
				"\n7 shares listed\n",
			want: []ShareInfo1{
				{Name: "Team Files", Type: smbTypeDisk, Remark: "Quarterly  planning"},
				{Name: "Team  Disk Files", Type: smbTypeDisk, Remark: "double space in the name"},
				{Name: "Résumé archives", Type: smbTypeDisk, Remark: "UTF-8 name"},
				{Name: "media", Type: smbTypeDisk, Remark: "Disk    in the comment"},
				{Name: "Office Printer", Type: smbTypePrint, Remark: "shared printer"},
				{Name: "IPC$", Type: smbTypeIPC | smbTypeSpecial},
				{Name: "Hidden Files$", Type: smbTypeDisk | smbTypeSpecial},
			},
		},
		{
			name: "share name fills or exceeds its column",
			output: header +
				row(fullWidth, "Disk", "exact width") +
				row(longName, "Disk", "overflow") +
				row(longHidden, "Disk", "") +
				row(ambiguous, "Disk", "") +
				row(strings.Repeat("x", 47)+" longer name", "Printer", ""),
			want: []ShareInfo1{
				{Name: fullWidth, Type: smbTypeDisk, Remark: "exact width"},
				{Name: longName, Type: smbTypeDisk, Remark: "overflow"},
				{Name: longHidden, Type: smbTypeDisk | smbTypeSpecial},
				{Name: strings.Repeat("x", 47) + " longer name", Type: smbTypePrint},
			},
		},
		{
			name: "unknown and unsupported types stay filtered",
			output: header +
				row("weird share", "Pipe", "") +
				row("another share", "Unknown", ""),
			want: []ShareInfo1{
				{Name: "weird share", Type: smbTypeDevice},
				{Name: "another share", Type: smbTypeDevice},
			},
		},
		{
			name:   "host with no shares",
			output: header + "\n0 shares listed\n",
		},
		{
			name: "empty output",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSMBUtilView(tt.output)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSMBUtilView() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

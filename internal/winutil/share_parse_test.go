package winutil

import (
	"reflect"
	"testing"
)

func TestParseSMBUtilView(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []ShareInfo1
	}{
		{
			name: "real smbutil view table",
			output: "Share                                           Type    Comment\n" +
				"----------------------------                    ----    -------\n" +
				"backups                                         Disk    nightly backups\n" +
				"media                                           Disk\n" +
				"IPC$                                            IPC\n" +
				"print$                                          Printer\n",
			want: []ShareInfo1{
				{Name: "backups", Type: smbTypeDisk, Remark: "nightly backups"},
				{Name: "media", Type: smbTypeDisk, Remark: ""},
				{Name: "IPC$", Type: smbTypeIPC | smbTypeSpecial, Remark: ""},
				{Name: "print$", Type: smbTypePrint | smbTypeSpecial, Remark: ""},
			},
		},
		{
			name:   "host with no shares",
			output: "Share             Type    Comment\n-----------------------------\n",
			want:   nil,
		},
		{
			name:   "empty output",
			output: "",
			want:   nil,
		},
		{
			name: "unknown type maps to device",
			output: "Share             Type    Comment\n" +
				"---------------------------\n" +
				"weirdshare        Pipe\n",
			want: []ShareInfo1{
				{Name: "weirdshare", Type: smbTypeDevice, Remark: ""},
			},
		},
		{
			name: "admin share gets special bit",
			output: "Share             Type    Comment\n" +
				"---------------------------\n" +
				"ADMIN$            Disk\n",
			want: []ShareInfo1{
				{Name: "ADMIN$", Type: smbTypeDisk | smbTypeSpecial, Remark: ""},
			},
		},
		{
			name: "multi-word comment preserved",
			output: "Share             Type    Comment\n" +
				"---------------------------\n" +
				"docs              Disk    team shared documents v2\n",
			want: []ShareInfo1{
				{Name: "docs", Type: smbTypeDisk, Remark: "team shared documents v2"},
			},
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

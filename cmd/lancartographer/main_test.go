package main

import (
	"slices"
	"testing"
)

func TestWantsCLI(t *testing.T) {
	cases := []struct {
		name     string
		exeBase  string
		args     []string
		wantCLI  bool
		wantArgs []string
	}{
		{"GUI exe, no args", "LanCartographer.exe", nil, false, nil},
		{"GUI exe with flags stays GUI", "LanCartographer.exe", []string{"--range", "192.168.1.0/24"}, false, []string{"--range", "192.168.1.0/24"}},
		{"CLI exe, no args", "lancartographer-cli.exe", nil, true, nil},
		{"CLI exe keeps args", "lancartographer-cli.exe", []string{"--range", "10.0.0.0/24"}, true, []string{"--range", "10.0.0.0/24"}},
		{"CLI exe case-insensitive", "LANCARTOGRAPHER-CLI.EXE", []string{"--json", "out.json"}, true, []string{"--json", "out.json"}},
		{"CLI token on GUI exe", "LanCartographer.exe", []string{"--cli"}, true, nil},
		{"CLI token stripped", "LanCartographer.exe", []string{"--cli", "--range", "192.168.1.0/24"}, true, []string{"--range", "192.168.1.0/24"}},
		{"CLI token on CLI exe", "lancartographer-cli.exe", []string{"--cli", "--json", "inv.json"}, true, []string{"--json", "inv.json"}},
		{"--cli not first is not the token", "LanCartographer.exe", []string{"--json", "--cli"}, false, []string{"--json", "--cli"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCLI, gotArgs := wantsCLI(tc.exeBase, tc.args)
			if gotCLI != tc.wantCLI {
				t.Errorf("wantsCLI(%q, %q) cli = %v, want %v", tc.exeBase, tc.args, gotCLI, tc.wantCLI)
			}
			if !slices.Equal(gotArgs, tc.wantArgs) {
				t.Errorf("wantsCLI(%q, %q) args = %q, want %q", tc.exeBase, tc.args, gotArgs, tc.wantArgs)
			}
		})
	}
}

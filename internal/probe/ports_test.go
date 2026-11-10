package probe

import (
	"slices"
	"testing"
)

func TestParsePorts(t *testing.T) {
	ports, err := ParsePorts("80, 443,80, 8080")
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{80, 443, 80, 8080}; !slices.Equal(ports, want) {
		t.Errorf("ParsePorts = %v, want %v", ports, want)
	}
	for _, input := range []string{"", "0", "65536", "abc", "80,x", "80,", "80,,443"} {
		if _, err := ParsePorts(input); err == nil {
			t.Errorf("ParsePorts(%q) should fail", input)
		}
	}
}

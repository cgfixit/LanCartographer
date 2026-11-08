package export

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWriteFileKeepsPriorInventoryWhenReplacementIsLocked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inventory.json")
	prior := []byte("prior inventory")
	if err := os.WriteFile(path, prior, 0o600); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	var handle windows.Handle
	writeErr := writeFile(path, func(w io.Writer) error {
		var err error
		handle, err = windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ,
			nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "new inventory")
		return err
	})
	if handle == 0 || handle == windows.InvalidHandle {
		t.Fatalf("fixture could not lock the destination: %v", writeErr)
	}
	defer windows.CloseHandle(handle)
	if writeErr == nil {
		t.Fatal("replacement unexpectedly succeeded while destination was locked")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, prior) {
		t.Fatalf("prior export changed: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("staging file left behind: %v, %v", entries, err)
	}
}

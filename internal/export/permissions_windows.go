package export

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func restrictReplacement(staged *os.File, path string) error {
	const daclInfo = windows.DACL_SECURITY_INFORMATION
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, daclInfo)
	if errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
		return nil
	}
	if err != nil {
		return &os.PathError{Op: "read export ACL", Path: path, Err: err}
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return &os.PathError{Op: "read export ACL", Path: path, Err: err}
	}
	// Apply the destination ACL before writing inventory to the staging file.
	if err := windows.SetNamedSecurityInfo(staged.Name(), windows.SE_FILE_OBJECT,
		daclInfo|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return &os.PathError{Op: "protect staged export", Path: path, Err: err}
	}
	return nil
}

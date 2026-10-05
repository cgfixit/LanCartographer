package winutil

import "errors"

// ErrAccessDenied is returned (possibly wrapped) by NetShareEnum when the
// remote host rejects the current credentials (ERROR_ACCESS_DENIED). A null
// session failing this way is a normal scan result, not an error condition.
var ErrAccessDenied = errors.New("access denied")

// ShareInfo1 is one share as reported by NetShareEnum at info level 1:
// name, type bitmask, and comment. No file contents are ever enumerated.
type ShareInfo1 struct {
	Name   string
	Type   uint32
	Remark string
}

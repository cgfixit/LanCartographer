package macdiag

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

var ErrOutputLimit = errors.New("diagnostic output limit exceeded")

type output struct {
	data   bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (b *output) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.data.Len() {
		b.cancel()
		return 0, ErrOutputLimit
	}
	return b.data.Write(p)
}

// Run waits for an app-owned helper and bounds each output stream separately.
func Run(parent context.Context, limit int, path string, args ...string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	out, warnings := &output{limit: limit, cancel: cancel}, &output{limit: limit, cancel: cancel}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	cmd.Stdout, cmd.Stderr = out, warnings
	cmd.WaitDelay = 500 * time.Millisecond
	err = cmd.Run()
	switch {
	case parent.Err() != nil:
		return nil, nil, parent.Err()
	case ctx.Err() != nil:
		return nil, nil, ErrOutputLimit
	default:
		return out.data.Bytes(), warnings.data.Bytes(), err
	}
}

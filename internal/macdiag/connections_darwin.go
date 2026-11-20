package macdiag

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

func ReadConnections(parent context.Context) (Snapshot, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	stdout, stderr, err := Run(ctx, 1<<20, "/usr/sbin/lsof", "-nP", "-iTCP", "-iUDP", "-F0pcftPnT")
	return connectionSnapshot(stdout, stderr, err)
}

func connectionSnapshot(stdout, stderr []byte, err error) (Snapshot, error) {
	warnings := len(bytes.TrimSpace(stderr)) > 0
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return Snapshot{}, err
	}
	snapshot := parseConnections(stdout)
	snapshot.Partial = snapshot.Partial || warnings || (err != nil && len(stdout) > 0)
	if snapshot.Partial && len(snapshot.Rows) == 0 {
		return snapshot, errors.New("lsof did not return a readable socket snapshot")
	}
	return snapshot, nil
}

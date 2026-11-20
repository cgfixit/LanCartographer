package macdiag

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestConnectionsEmptyAndPartial(t *testing.T) {
	exit1 := exec.Command("/usr/bin/false").Run()
	if got, err := connectionSnapshot(nil, nil, exit1); err != nil || got.Partial || len(got.Rows) != 0 {
		t.Fatalf("empty observation: %+v %v", got, err)
	}
	if _, err := connectionSnapshot(nil, []byte("permission denied"), exit1); err == nil {
		t.Fatal("permission failure reported as an empty observation")
	}
	data := []byte(processRecord + "f3\x00tIPv4\x00PTCP\x00n127.0.0.1:80\x00TST=LISTEN\x00\n")
	if got, err := connectionSnapshot(data, []byte("warning"), exit1); err != nil || !got.Partial || len(got.Rows) != 1 {
		t.Fatalf("partial observation: %+v %v", got, err)
	}
}

func TestDiagnosticCommandCancelsAndBounds(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if _, _, err := Run(ctx, 1024, "/bin/sleep", "10"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
	for _, args := range [][]string{{"/usr/bin/yes"}, {"/bin/sh", "-c", "exec /usr/bin/yes >&2"}} {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		_, _, err := Run(ctx, 1024, args[0], args[1:]...)
		cancel()
		if !errors.Is(err, ErrOutputLimit) {
			t.Fatalf("output cap = %v", err)
		}
	}
}

package tasks

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Off Windows there is no pipe namespace, so the enumerator must return nil
// (not an empty list, not an error) — that is what keeps named_pipe_detection
// out of a non-Windows report. On Windows it must return the real namespace:
// asserted by the presence of pipes every Windows host has, never by a count —
// Winsock2\CatalogChangeListener entries are per-process and the total moves
// between runs.
func TestListNamedPipes(t *testing.T) {
	pipes, err := ListNamedPipes()

	if runtime.GOOS != "windows" {
		if err != nil || pipes != nil {
			t.Fatalf("ListNamedPipes() = %v, %v; want nil, nil off Windows", pipes, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("ListNamedPipes() error: %v", err)
	}
	for _, want := range []string{`\\.\pipe\InitShutdown`, `\\.\pipe\lsass`, `\\.\pipe\ntsvcs`} {
		if !slices.Contains(pipes, want) {
			t.Errorf("well-known pipe %q missing from %d enumerated pipes", want, len(pipes))
		}
	}
	for _, p := range pipes {
		if !strings.HasPrefix(p, `\\.\pipe\`) || p == `\\.\pipe\` {
			t.Errorf("pipe %q is not a full \\\\.\\pipe\\<name> path", p)
		}
	}
}

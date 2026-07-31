//go:build !windows
// +build !windows

package tasks

import (
	"errors"
	"time"
)

// ListNamedPipes is Windows-only — no other OS has a named-pipe namespace.
// Returning nil rather than an error is what keeps named_pipe_detection out of
// a non-Windows report entirely: the caller emits the finding only for a
// non-nil list.
func ListNamedPipes() ([]string, error) { return nil, nil }

// errNoPipeNamespace is what the seeding half of the pipe seam reports off
// Windows. Unreachable in production — pipe targets are windows-scoped and
// ListTargets filters to the running OS — so if it ever fires it is one skipped
// entry, not a failed run.
var errNoPipeNamespace = errors.New("named pipes are Windows-only")

func ServePipe(string, time.Duration) error { return errNoPipeNamespace }

func seedPipe(string) (seededPipe, error) { return seededPipe{}, errNoPipeNamespace }

func processCreated(int) (int64, error) { return 0, errNoPipeNamespace }

func removePipeDecoy(seededPipe) bool { return false }

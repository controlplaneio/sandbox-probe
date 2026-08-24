//go:build !windows
// +build !windows

package tasks

import (
	"time"
)

// ListNamedPipes is Windows-only — no other OS has a named-pipe namespace.
// Returning nil rather than an error is what keeps named_pipe_detection out of
// a non-Windows report entirely: the caller emits the finding only for a
// non-nil list.
func ListNamedPipes() ([]string, error) { return nil, nil }

func ServePipe(string, time.Duration) error { return errNoPipeNamespace }

func seedPipe(string) (seededPipe, error) { return seededPipe{}, errNoPipeNamespace }

func processCreated(int) (int64, error) { return 0, errNoPipeNamespace }

func removePipeDecoy(seededPipe) bool { return false }

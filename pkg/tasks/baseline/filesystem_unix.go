//go:build !windows
// +build !windows

package tasks

import (
	"os"

	"golang.org/x/sys/unix"
)

func isReadable(path string) bool {
	if err := unix.Access(path, unix.R_OK); err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isWritable(path string) bool {
	if err := unix.Access(path, unix.W_OK); err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

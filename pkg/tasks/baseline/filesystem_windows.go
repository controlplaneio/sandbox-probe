//go:build windows
// +build windows

package tasks

import "os"

// On Windows we don't have POSIX access(2). The cheapest reliable signal
// is to open the file with the requested access and report success/failure;
// the OS evaluates the ACL during the open call.
func isReadable(path string) bool {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

//go:build windows
// +build windows

package tasks

import (
	"os"
	"syscall"
)

// On Windows we don't have POSIX access(2). The cheapest reliable signal
// is to open the file with the requested access and report success/failure;
// the OS evaluates the ACL during the open call.
func isReadable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		// For directories, try to read the directory contents
		_, err := os.ReadDir(path)
		return err == nil
	}

	// For files, try to open
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if info.IsDir() {
		// For directories, try to open with write access using syscall
		// Need FILE_FLAG_BACKUP_SEMANTICS to open directories on Windows
		pathPtr, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return false
		}
		handle, err := syscall.CreateFile(
			pathPtr,
			syscall.GENERIC_WRITE,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
			nil,
			syscall.OPEN_EXISTING,
			syscall.FILE_FLAG_BACKUP_SEMANTICS,
			0,
		)
		if err != nil {
			return false
		}
		syscall.CloseHandle(handle)
		return true
	}

	// For files, try to open with write access
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

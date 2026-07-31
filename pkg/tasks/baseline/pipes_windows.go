//go:build windows
// +build windows

package tasks

import (
	"errors"
	"syscall"
)

// pipePrefix is the Win32 path of the named-pipe object namespace. The
// namespace is machine-global (unlike \Sessions\<n>\BaseNamedObjects), so what
// a scan sees does not depend on the session it runs in.
const pipePrefix = `\\.\pipe\`

// ListNamedPipes returns the named pipes currently in the pipe namespace, as
// full `\\.\pipe\<name>` paths — the form the IPC target catalogue uses, and
// the form CreateFile takes. Names may themselves contain a backslash
// (`Winsock2\CatalogChangeListener-3a4-0`): the namespace is not flat.
//
// A plain FindFirstFileW/FindNextFileW loop is all this needs — verified on a
// real Windows 11 host as an ordinary unelevated standard user, which is why
// there is no ntdll import, no NTSTATUS handling and no new dependency here
// (issue #11, and the correction in ADR 0002). Enumerating the directory is
// not the same as opening a pipe in it: a standard user is still denied on
// \\.\pipe\lsass, so a finding means "could list the names", nothing more.
func ListNamedPipes() ([]string, error) {
	pattern, err := syscall.UTF16PtrFromString(pipePrefix + "*")
	if err != nil {
		return nil, err
	}
	var data syscall.Win32finddata
	h, err := syscall.FindFirstFile(pattern, &data)
	if err != nil {
		return nil, err
	}
	defer syscall.FindClose(h)

	pipes := []string{}
	for {
		pipes = append(pipes, pipePrefix+syscall.UTF16ToString(data.FileName[:]))
		if err := syscall.FindNextFile(h, &data); err != nil {
			if errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
				return pipes, nil
			}
			return nil, err
		}
	}
}

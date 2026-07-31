//go:build windows
// +build windows

package tasks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
)

// pipeServerStartTimeout is how long seeding waits for a spawned server to show
// up in the namespace before giving up on it. A decoy the scan cannot see yet
// is a decoy that was never planted, so the wait is what makes "seed then scan"
// deterministic rather than a race with process startup.
const pipeServerStartTimeout = 15 * time.Second

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

// ServePipe creates the named pipe name and holds it open for d, then exits —
// the whole of a pipe decoy. A pipe exists only while a server holds a handle
// to it, so nothing here waits for a client: the scan lists names and never
// connects, exactly as the socket scan only stat()s. d is the belt of the
// live-artifact lifecycle, so an unreachable cleanup is not a permanent leak.
//
// FILE_FLAG_FIRST_PIPE_INSTANCE is the soft-plant rule made atomic: if a real
// service already serves this name, creation fails rather than adding an
// instance beside it, so a decoy can never end up answering for a real tool.
func ServePipe(name string, d time.Duration) error {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	h, err := windows.CreateNamedPipe(p,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE, 1, 512, 512, 0, nil)
	if err != nil {
		return fmt.Errorf("creating pipe %s: %w", name, err)
	}
	defer windows.CloseHandle(h)
	time.Sleep(d)
	return nil
}

// pipeExists reports whether name is in the pipe namespace now, asked of the
// same enumeration the scan uses — so the seeder's "is this taken" and the
// probe's "can I see it" can never disagree. Pipe names are case-insensitive.
func pipeExists(name string) bool {
	pipes, err := ListNamedPipes()
	if err != nil {
		return false
	}
	return slices.ContainsFunc(pipes, func(p string) bool { return strings.EqualFold(p, name) })
}

// seedPipe soft-plants a decoy pipe by spawning the probe itself as a server
// for it, and returns the record cleanup needs. Soft: a name already served is
// left alone. The recorded identity is the pid *and* its creation time, which
// is what lets cleanup refuse to signal a pid Windows has since handed to
// something else — the pipe's own name cannot serve as that check, since by
// then it may belong to the real tool.
func seedPipe(name string) (seededPipe, error) {
	if pipeExists(name) {
		return seededPipe{}, errOccupied
	}
	exe, err := os.Executable()
	if err != nil {
		return seededPipe{}, err
	}
	cmd := exec.Command(exe, "serve-pipe", name, decoyProcessLifetime.String())
	if err := cmd.Start(); err != nil {
		return seededPipe{}, err
	}
	pid := cmd.Process.Pid
	for deadline := time.Now().Add(pipeServerStartTimeout); !pipeExists(name); {
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			return seededPipe{}, fmt.Errorf("pipe %s never appeared; its server exited or the name was taken", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
	created, err := processCreated(pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return seededPipe{}, err
	}
	return seededPipe{Name: name, PID: pid, Created: created}, nil
}

// processCreated is a pid's creation time, the one part of its identity Windows
// never reuses.
func processCreated(pid int) (int64, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0, err
	}
	return creation.Nanoseconds(), nil
}

// removePipeDecoy terminates the recorded pipe server — and only it. A pid that
// is gone, or one whose creation time no longer matches the record, belongs to
// something else by now and is left running, so a reused pid can never cost an
// unrelated process. Waiting on the handle afterwards is what makes the pipe
// provably gone when cleanup returns, rather than shortly after.
func removePipeDecoy(p seededPipe) bool {
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE,
		false, uint32(p.PID))
	if err != nil {
		return false // already gone
	}
	defer windows.CloseHandle(h)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil ||
		creation.Nanoseconds() != p.Created {
		log.Debug().Int("pid", p.PID).Str("pipe", p.Name).
			Msg("Recorded pipe server is gone or is no longer ours; terminating nothing")
		return false
	}
	if err := windows.TerminateProcess(h, 1); err != nil {
		return false
	}
	_, _ = windows.WaitForSingleObject(h, uint32(pipeServerStartTimeout.Milliseconds()))
	return true
}

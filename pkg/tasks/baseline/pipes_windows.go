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

const (
	pipeInstanceBufSize = 512
	// knockAttempts and knockInterval bound the expiry wake-up below. One knock is
	// normally enough; repeating means a knock that itself fails cannot strand the
	// server past its lifetime. The belt is a safety property, not an optimisation.
	knockAttempts = 5
	knockInterval = time.Second
)

// createPipeInstance creates one instance of name. first carries
// FILE_FLAG_FIRST_PIPE_INSTANCE — the soft-plant rule made atomic: if a real service
// already serves this name, creation fails rather than adding an instance beside it, so a
// decoy can never end up answering for a real tool. Every instance AFTER ours must not ask
// for it, because we hold one and it would fail against ourselves.
func createPipeInstance(name string, first bool) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return windows.InvalidHandle, err
	}
	mode := uint32(windows.PIPE_ACCESS_DUPLEX)
	if first {
		mode |= windows.FILE_FLAG_FIRST_PIPE_INSTANCE
	}
	h, err := windows.CreateNamedPipe(p, mode,
		windows.PIPE_TYPE_BYTE, windows.PIPE_UNLIMITED_INSTANCES,
		pipeInstanceBufSize, pipeInstanceBufSize, 0, nil)
	if err != nil {
		return windows.InvalidHandle, fmt.Errorf("creating pipe %s: %w", name, err)
	}
	return h, nil
}

// ServePipe serves name until d expires, answering each client with the pipe's own name and
// then recycling the instance.
//
// This replaces a create-and-sleep server. The scan no longer only LISTS names: enumeration
// was measured not to discriminate — 57 pipes confined and 57 unconfined on a Windows 11
// host, 40 and 40 on a GitHub runner — because listing \\.\pipe\* is a directory read that a
// restricted token does not change. Reachability is where the signal is, and it needs a
// server that recycles rather than one that holds a single instance and sleeps.
//
// The token written back is the pipe's own name. A client that reads it has provably reached
// THIS server rather than merely a name that resolved, which is the gap a redirected object
// namespace — a server silo, an AppContainer — can otherwise hide.
func ServePipe(name string, d time.Duration) error {
	h, err := createPipeInstance(name, true)
	if err != nil {
		return err
	}
	defer func() {
		if h != windows.InvalidHandle {
			_ = windows.CloseHandle(h)
		}
	}()

	// The belt has to fire while the loop below is blocked in ConnectNamedPipe. ServePort
	// closes its listener to unblock Accept; CloseHandle on a handle with a synchronous
	// ConnectNamedPipe in flight is not the documented equivalent, so the pipe answer is to
	// become a client of our own pipe — the very client reachability already needs.
	//
	// It must be readPipeToken and not a bare open: a client that connects and never drains
	// would leave FlushFileBuffers below waiting on it, which is the same wedge in a new place.
	deadline := time.Now().Add(d)
	go func() {
		time.Sleep(d)
		for range knockAttempts {
			_, _ = readPipeToken(name)
			time.Sleep(knockInterval)
		}
	}()

	for time.Now().Before(deadline) {
		// A client that arrived between creation and here is already connected, and
		// ERROR_PIPE_CONNECTED says exactly that. It is success, not failure.
		if cErr := windows.ConnectNamedPipe(h, nil); cErr != nil &&
			!errors.Is(cErr, windows.ERROR_PIPE_CONNECTED) {
			return nil // the instance went; there is nothing left to serve
		}
		var n uint32
		_ = windows.WriteFile(h, []byte(name), &n, nil)
		// Returns once the client has drained it, or once the client goes away.
		// ponytail: a client that connects and never reads wedges the loop until the
		// lifetime expires. The process dies then anyway, and the only client this private
		// name ever has is ours.
		_ = windows.FlushFileBuffers(h)

		// Create the replacement BEFORE closing the served instance, so the name never
		// lapses out of the namespace between clients — a scan enumerating at that instant
		// must not miss it.
		next, nErr := createPipeInstance(name, false)
		_ = windows.CloseHandle(h)
		h = windows.InvalidHandle // the defer must not close it a second time
		if nErr != nil {
			return nErr
		}
		h = next
	}
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

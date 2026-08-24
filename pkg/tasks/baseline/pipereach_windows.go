//go:build windows
// +build windows

package tasks

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// classifyWin32 is classify()'s sibling for the Win32 error space.
//
// It is a separate function rather than an extension of the helpers in errno_windows.go,
// and the reason is a trap that compiles cleanly. Those helpers match Winsock numbers
// (10000+); a CreateFile failure carries a low Win32 code. The two spaces do not collide, so
// every predicate there returns false for every pipe error and classify() falls through to
// OutcomeProbeError — including ERROR_ACCESS_DENIED, the single most interesting result,
// reported as a malfunction of the probe rather than as the sandbox doing its job.
func classifyWin32(err error) (Outcome, string, string) {
	if err == nil {
		return OutcomeReachable, "", ""
	}

	var se *os.SyscallError
	call := ""
	if errors.As(err, &se) {
		call = se.Syscall
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		return win32Outcome(errno), call, win32Name(errno)
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return OutcomeSilent, call, "deadline"
	}
	return OutcomeProbeError, call, ""
}

// win32Errors is the single table behind both the outcome and the stable name of a Win32
// error from a pipe open. One table rather than two switches over the same constants: they
// have to agree, and two switches that must agree are two switches that will eventually not.
//
// Every one of these codes is FORGEABLE. A filesystem minifilter or an object-manager filter
// chooses what it returns, exactly as iptables can synthesise a TCP reset. So the rule from
// reach.go transfers unweakened: only OutcomeReachable is scored, and it is reached only by a
// completed token round trip. Everything here is diagnostic.
var win32Errors = map[syscall.Errno]struct {
	outcome Outcome
	name    string
}{
	// The name did not resolve, so no access check was ever reached. That is "no route", not
	// "denied" — and it is also what a private object namespace (a server silo, an
	// AppContainer) looks like from the inside.
	windows.ERROR_FILE_NOT_FOUND: {OutcomeUnreachable, "ERROR_FILE_NOT_FOUND"},
	windows.ERROR_PATH_NOT_FOUND: {OutcomeUnreachable, "ERROR_PATH_NOT_FOUND"},
	windows.ERROR_BAD_NETPATH:    {OutcomeUnreachable, "ERROR_BAD_NETPATH"},

	// The name resolved and the access check FAILED. A restricted token or a DACL denial lands
	// exactly here. This is the case the Winsock helpers lose.
	windows.ERROR_ACCESS_DENIED: {OutcomeBlocked, "ERROR_ACCESS_DENIED"},

	// The wait expired with nothing back. Ambiguous by construction.
	windows.ERROR_SEM_TIMEOUT: {OutcomeSilent, "ERROR_SEM_TIMEOUT"},

	// NPFS answered: every instance is in use. A real refusal from a real server, and forgeable
	// like the rest. Against our own decoy — unlimited instances, served serially — it should
	// never appear, so seeing it is itself a signal.
	windows.ERROR_PIPE_BUSY: {OutcomeRefused, "ERROR_PIPE_BUSY"},

	// None of these can come from a CreateFile against a live server. They mean our own server
	// went away mid-round-trip, or we built a bad name: facts about the probe, never about the
	// sandbox. That distinction is the one the old port scanner lost.
	windows.ERROR_PIPE_NOT_CONNECTED: {OutcomeProbeError, "ERROR_PIPE_NOT_CONNECTED"},
	windows.ERROR_BROKEN_PIPE:        {OutcomeProbeError, "ERROR_BROKEN_PIPE"},
	windows.ERROR_NO_DATA:            {OutcomeProbeError, "ERROR_NO_DATA"},
	windows.ERROR_INVALID_NAME:       {OutcomeProbeError, "ERROR_INVALID_NAME"},
	windows.ERROR_NOT_SUPPORTED:      {OutcomeProbeError, "ERROR_NOT_SUPPORTED"},
	windows.ERROR_OPERATION_ABORTED:  {OutcomeProbeError, "ERROR_OPERATION_ABORTED"},
}

// win32Outcome and win32Name read that table. An error the table has no opinion on is not
// silence and is not a verdict, so it says so rather than guessing; its name falls back to the
// OS message, which is localised and unfit to compare across runs — anything worth reasoning
// about belongs in the table.
func win32Outcome(e syscall.Errno) Outcome {
	if v, ok := win32Errors[e]; ok {
		return v.outcome
	}
	return OutcomeProbeError
}

func win32Name(e syscall.Errno) string {
	if v, ok := win32Errors[e]; ok {
		return v.name
	}
	return e.Error()
}

// readPipeToken opens name as a CLIENT and reads back what the server writes.
//
// This is a client connect. It consumes a server instance and delivers a connection event,
// so it is only ever pointed at a name this probe serves itself — ReachPipeName or the
// per-pid control. Never at a catalogue name; see ReachPipeName for why that distinction
// cannot be made safely at scan time.
func readPipeToken(name string) (string, error) {
	if name == "" {
		return "", os.ErrInvalid
	}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		// Wrapped so classifyWin32 recovers the syscall name through the same errors.As
		// walk classify() uses on net's errors.
		return "", &os.SyscallError{Syscall: "CreateFile", Err: err}
	}
	defer windows.CloseHandle(h)

	buf := make([]byte, len(name))
	done := make(chan error, 1)
	go func() {
		for got := 0; got < len(buf); {
			var n uint32
			if rErr := windows.ReadFile(h, buf[got:], &n, nil); rErr != nil {
				done <- &os.SyscallError{Syscall: "ReadFile", Err: rErr}
				return
			}
			if n == 0 {
				done <- &os.SyscallError{Syscall: "ReadFile", Err: windows.ERROR_BROKEN_PIPE}
				return
			}
			got += int(n)
		}
		done <- nil
	}()

	select {
	case rErr := <-done:
		if rErr != nil {
			return "", rErr
		}
		return string(buf), nil
	case <-time.After(probeDeadline):
		// Cancel rather than leak a read against a handle the defer is about to close, then
		// wait for the reader to actually be gone.
		_ = windows.CancelIoEx(h, nil)
		<-done
		return "", os.ErrDeadlineExceeded
	}
}

// probeOwnPipe attempts one round trip against a name this probe owns.
func probeOwnPipe(name string) (Outcome, string, string) {
	got, err := readPipeToken(name)
	if err != nil {
		return classifyWin32(err)
	}
	if !strings.EqualFold(got, name) {
		// Something opened, something answered, and it was not us. That is not reach: the
		// measurement did not conclude, and saying so beats scoring a stranger's pipe.
		return OutcomeProbeError, "ReadFile", "token_mismatch"
	}
	return OutcomeReachable, "", ""
}

// createCheck creates — and immediately destroys — one pipe under a name only this process
// can hold. Safe by construction: the name carries this pid, FILE_FLAG_FIRST_PIPE_INSTANCE
// means creation fails rather than joining anything, and the handle is closed before this
// returns.
func createCheck() (string, error) {
	name := pipePrefix + "sandbox-probe-create-check-" + strconv.Itoa(os.Getpid())
	h, err := createPipeInstance(name, true)
	if err != nil {
		return "", err
	}
	_ = windows.CloseHandle(h)
	return name, nil
}

// MeasurePipeReach is the Windows named-pipe reachability measurement.
//
// SAFETY, and the reason this function is short: it opens exactly two names, both of which
// only this probe can own. Opening a REAL service's pipe is not passive — it consumes a
// server instance, delivers a connection event, and can hang or break a badly written
// server. Reading a foreign pipe's DACL by name is a client connect too, per Microsoft's own
// documentation, so there is no passive alternative. The catalogue names are NEVER probed.
//
// NEVER add ImpersonateNamedPipeClient. That is the attack, not the measurement.
func MeasurePipeReach() PipeReach {
	out := PipeReach{Status: map[string]any{"method": "own-decoy-roundtrip-v1"}}

	// Calibrate BEFORE probing the decoy, the same order and for the same reason as
	// calibrate() in localsvc.go: a name nothing ever serves says what "absent" looks like
	// on THIS host, so a not-found against the decoy is interpretable.
	c, _, cErrno := probeOwnPipe(reachControlPipeName())
	out.Status["control"] = string(c)
	if cErrno != "" {
		out.Status["control_errno"] = cErrno
	}

	// Enumeration is a DIFFERENT access check from opening. Recording whether the decoy is
	// even visible is what separates "the sandbox hid it" from "nobody seeded it", and it
	// is the gate on whether anything is claimed at all.
	pipes, err := ListNamedPipes()
	if err != nil {
		out.Status["namespace"] = "denied"
		out.Status["error"] = err.Error()
	} else {
		out.Status["namespace"] = "read"
		out.Status["pipes_found"] = float64(len(pipes))
		out.Status["decoy_enumerated"] = slices.ContainsFunc(pipes,
			func(p string) bool { return strings.EqualFold(p, ReachPipeName) })
	}

	o, call, errno := probeOwnPipe(ReachPipeName)
	row := map[string]any{"pipe": ReachPipeName, "outcome": string(o)}
	if call != "" {
		row["syscall"] = call
	}
	if errno != "" {
		row["errno"] = errno
	}
	out.Status["results"] = []any{row} // structpb takes the basic Go kinds only

	out.Reached = []string{}
	if o == OutcomeReachable {
		out.Reached = append(out.Reached, ReachPipeName)
	}

	// Creation is the third access check. A filter can allow enumeration and deny
	// CreateNamedPipe, so it is measured rather than inferred from the other two.
	if name, cErr := createCheck(); cErr == nil {
		out.Created = name
		out.Status["creation"] = "ok"
	} else {
		co, _, cn := classifyWin32(cErr)
		out.Status["creation"] = string(co)
		out.Status["creation_errno"] = cn
	}
	return out
}

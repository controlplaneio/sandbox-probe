//go:build windows
// +build windows

package tasks

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// The Win32 error space, mapped. Every code here is forgeable by a filesystem minifilter or
// an object-manager filter, which is why only OutcomeReachable is ever scored — but the
// distinctions still have to be right, because they are what a reader uses to tell a policy
// denial from a probe malfunction.
func TestClassifyWin32Mapping(t *testing.T) {
	for _, tt := range []struct {
		errno syscall.Errno
		want  Outcome
		name  string
	}{
		{windows.ERROR_FILE_NOT_FOUND, OutcomeUnreachable, "ERROR_FILE_NOT_FOUND"},
		{windows.ERROR_PATH_NOT_FOUND, OutcomeUnreachable, "ERROR_PATH_NOT_FOUND"},
		{windows.ERROR_BAD_NETPATH, OutcomeUnreachable, "ERROR_BAD_NETPATH"},
		{windows.ERROR_ACCESS_DENIED, OutcomeBlocked, "ERROR_ACCESS_DENIED"},
		{windows.ERROR_SEM_TIMEOUT, OutcomeSilent, "ERROR_SEM_TIMEOUT"},
		{windows.ERROR_PIPE_BUSY, OutcomeRefused, "ERROR_PIPE_BUSY"},
		{windows.ERROR_PIPE_NOT_CONNECTED, OutcomeProbeError, "ERROR_PIPE_NOT_CONNECTED"},
		{windows.ERROR_BROKEN_PIPE, OutcomeProbeError, "ERROR_BROKEN_PIPE"},
		{windows.ERROR_NO_DATA, OutcomeProbeError, "ERROR_NO_DATA"},
		{windows.ERROR_OPERATION_ABORTED, OutcomeProbeError, "ERROR_OPERATION_ABORTED"},
		// An error we have no opinion on is not silence and is not a verdict.
		{windows.ERROR_INVALID_HANDLE, OutcomeProbeError, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, call, name := classifyWin32(&os.SyscallError{Syscall: "CreateFile", Err: tt.errno})
			assert.Equal(t, tt.want, got)
			assert.Equal(t, "CreateFile", call, "the syscall name must survive the unwrap")
			if tt.name != "" {
				assert.Equal(t, tt.name, name, "the stable symbol, not a localised OS message")
			}
		})
	}
}

// The regression this whole file exists for.
//
// classify() matches Winsock numbers (10000+); a pipe open returns low Win32 codes. They do
// not collide, so classify() silently reports ERROR_ACCESS_DENIED — a sandbox denying access,
// the single most interesting result — as a malfunction of the probe. Asserting both sides is
// what stops someone "simplifying" the two functions into one.
func TestClassifyWin32SeparatesDeniedFromProbeError(t *testing.T) {
	denied := &os.SyscallError{Syscall: "CreateFile", Err: windows.ERROR_ACCESS_DENIED}

	got, _, _ := classify(denied)
	assert.Equal(t, OutcomeProbeError, got,
		"classify is Winsock-only and cannot read a Win32 code; if this ever changes, "+
			"revisit whether the two classifiers should still be separate")

	got, _, _ = classifyWin32(denied)
	assert.Equal(t, OutcomeBlocked, got,
		"a denied pipe open is the sandbox doing its job, not the probe failing")
}

// The rework's whole point: the decoy must answer more than one client. Today's
// create-and-sleep server serves at most one and never recycles the instance.
func TestServePipeAnswersRepeatedClients(t *testing.T) {
	name := testPipeName(t)
	served := make(chan error, 1)
	go func() { served <- ServePipe(name, 30*time.Second) }()
	require.True(t, waitForPipe(t, name, true), "the decoy server never came up")

	for i := range 3 {
		got, _, errno := probeOwnPipe(name)
		assert.Equalf(t, OutcomeReachable, got, "round trip %d failed with %s", i+1, errno)
		assert.Truef(t, pipeExists(name),
			"the name lapsed out of the namespace after round trip %d; a scan enumerating at "+
				"that instant would miss a decoy that is really there", i+1)
	}

	select {
	case err := <-served:
		t.Fatalf("the server exited early: %v", err)
	default:
	}
}

// The belt, and the direct answer to "does the lifetime still fire when the server is blocked
// waiting for a client that never comes?".
//
// Before the rework this passed trivially against time.Sleep(d). It is now the only thing
// standing between a blocking ConnectNamedPipe and a decoy that outlives every scan — a pipe
// server left running on a developer's laptop until reboot.
func TestServePipeExpiresWhileWaitingForAClient(t *testing.T) {
	name := testPipeName(t)
	done := make(chan error, 1)
	go func() { done <- ServePipe(name, 2*time.Second) }()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("ServePipe never returned with no client connecting: the self-expiry cannot " +
			"interrupt ConnectNamedPipe, so a decoy would outlive its lifetime")
	}
}

// The token check is load-bearing, not decorative. A successful open proves a name resolved
// and an access check passed; it does not prove WHICH object answered, and Windows redirects
// object namespaces for exactly the container-shaped isolation being measured.
func TestReachRejectsAForeignToken(t *testing.T) {
	name := testPipeName(t) + "-foreign"

	h, err := createPipeInstance(name, true)
	require.NoError(t, err)
	go func() {
		defer windows.CloseHandle(h)
		if cErr := windows.ConnectNamedPipe(h, nil); cErr != nil &&
			cErr != windows.ERROR_PIPE_CONNECTED {
			return
		}
		var n uint32
		_ = windows.WriteFile(h, []byte("not-the-name-you-asked-for"), &n, nil)
		_ = windows.FlushFileBuffers(h)
	}()
	require.True(t, waitForPipe(t, name, true))

	got, _, _ := probeOwnPipe(name)
	assert.NotEqual(t, OutcomeReachable, got,
		"something answered that was not our server, so the measurement did not conclude")
}

// End to end, and the last assertion is the one guarding the false-blocked class of bug.
func TestSeedPlantsAndScanReachesTheReachabilityDecoy(t *testing.T) {
	record := filepath.Join(t.TempDir(), "record.json")
	shortLifetime(t, 60*time.Second)
	t.Cleanup(func() { _, _ = CleanupSeeded(record) })

	// Deliberately no catalogue targets: the reachability decoy must be planted on its own.
	//
	// It is not counted in SeedResult — Planted and Skipped tally the caller's targets, and
	// this is an instrument the probe plants for itself. So an empty target list must report
	// nothing planted while still leaving a reachable decoy behind, and both halves of that
	// are asserted here.
	res, err := SeedTargets(nil, record)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Planted, "the probe's own instrument must not inflate the caller's tally")
	assert.Equal(t, 0, res.Skipped, "nor its skip count")

	names, ok := MeasurePipeReach().Names()
	assert.True(t, ok, "the decoy was planted, so the measurement must conclude")
	assert.Equal(t, []string{ReachPipeName}, names)

	_, err = CleanupSeeded(record)
	require.NoError(t, err)

	_, ok = MeasurePipeReach().Names()
	assert.False(t, ok,
		"with no decoy planted the answer must be unmeasured, not an empty list: publishing [] "+
			"would claim a sandbox blocked something that was never there")
}

// Creation is the third access check, measured rather than inferred.
func TestCreationIsMeasuredAndPidScoped(t *testing.T) {
	r := MeasurePipeReach()
	names, ok := r.CreatedPipe()
	require.True(t, ok, "the measurement did not conclude: %v", r.Status["creation_errno"])
	// An empty list is a legitimate answer — a sandbox denying CreateNamedPipe — so it must not
	// index-panic here. On an unconfined runner creation succeeds and the name carries the pid.
	require.Lenf(t, names, 1, "creation was denied on an unconfined runner: %v", r.Status["creation_errno"])
	assert.Contains(t, names[0], strconv.Itoa(os.Getpid()), "the name must be scoped to this process")
	assert.False(t, pipeExists(names[0]), "the check pipe must be closed before the call returns")
}

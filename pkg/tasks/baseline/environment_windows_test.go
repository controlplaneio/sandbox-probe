//go:build windows
// +build windows

package tasks

import (
	"fmt"
	"os"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// The badge's false case: with neither Windows token bit set and nothing else present, Windows must
// report no runtime at all.
//
// Two of the detectors ahead of it are environment-sensitive even on Windows, and both had to
// be neutralised for this to be deterministic. The `container` env var is the awkward one:
// os.Getenv is CASE-INSENSITIVE on Windows, because it resolves through
// GetEnvironmentVariableW, so a variable named CONTAINER or Container in any casing reaches
// the same check and promotes the answer to RuntimeUnknown. A GitHub windows-latest runner has
// one; a bare Windows 11 host does not, which is why this passed locally and failed in CI.
//
// The remaining detectors really are hard-false stubs here — isSeatbelt, probeForLandlock,
// isChroot and isProcSelfSetNoNewPrivs all come from the !darwin/!linux files — so with these
// two pinned, the two token bools are the only inputs left that can move the answer. That is
// what makes this a regression guard: it fails if anyone later wires a third Windows signal
// into the runtime chain without saying so.
func TestGetContainerRuntimeWindowsTokenBitsOnWindows(t *testing.T) {
	origToken, origAppC, origRead, origExists := isRestrictedToken, isAppContainer, readFile, fileExistsFunc
	t.Cleanup(func() {
		isRestrictedToken, isAppContainer, readFile, fileExistsFunc = origToken, origAppC, origRead, origExists
	})
	readFile = func(string) ([]byte, error) { return nil, fmt.Errorf("file not found") }
	fileExistsFunc = func(string) bool { return false }
	t.Setenv("container", "")

	for _, tt := range []struct {
		name                     string
		restricted, appContainer bool
		want                     ContainerRuntime
	}{
		{"neither", false, false, RuntimeNotFound},
		{"restricted token", true, false, RuntimeUnknown},
		{"app container", false, true, RuntimeUnknown},
		{"both", true, true, RuntimeUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isRestrictedToken = func() bool { return tt.restricted }
			isAppContainer = func() bool { return tt.appContainer }
			assert.Equal(t, tt.want, GetContainerRuntime(0, 0),
				"container=%q — a non-empty value here promotes the answer to RuntimeUnknown",
				os.Getenv("container"))
		})
	}
}

// tokenIsAppContainer is a hand-written constant because x/sys/windows does not export it.
// This checks it against x/sys's own enum, which stops one value earlier at TokenLogonSid.
//
// Two independent sources have to agree: the literal is TokenIsAppContainer from winnt.h, and
// TokenLogonSid+1 is where x/sys's iota run puts it. If a future x/sys inserts a class into
// that run, this fails rather than the probe silently querying a different token property and
// reading its result as a boolean.
func TestTokenIsAppContainerMatchesTheXSysEnum(t *testing.T) {
	assert.Equal(t, windows.TokenLogonSid+1, tokenIsAppContainer,
		"TOKEN_INFORMATION_CLASS 29 is one past TokenLogonSid; x/sys has moved the enum")
}

// The false-positive guard for the AppContainer detector, against the real API rather than a
// mock of it.
//
// The Go test binary is an ordinary console process, so it is not in an AppContainer, and the
// detector must say so. This is weaker than the UAC guard below only because AppContainer has
// no near-miss artefact to hand: nothing on a normal desktop resembles one. That is also the
// reason the detector is safe, and this pins the claim on a real Windows runner.
//
// The raw query is repeated here rather than only calling isAppContainerImpl, because that
// function swallows an error to false. Asserting on it alone would pass identically whether the
// class number is right and the answer is genuinely false, or the call failed outright and
// nothing was measured — the same absent-versus-empty confusion the findings themselves avoid.
// Requiring the syscall to succeed is what proves tokenIsAppContainer names a real class and
// that the probe can read it on this OS.
func TestIsAppContainerFalseForAnOrdinaryProcess(t *testing.T) {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		t.Fatalf("OpenProcessToken: %v", err)
	}
	defer func() { _ = tok.Close() }()

	var isAppC, retLen uint32
	err := windows.GetTokenInformation(tok, tokenIsAppContainer,
		(*byte)(unsafe.Pointer(&isAppC)), uint32(unsafe.Sizeof(isAppC)), &retLen)
	require.NoError(t, err,
		"TokenIsAppContainer (class %d) must be a queryable class on this Windows build", tokenIsAppContainer)
	assert.Equal(t, uint32(4), retLen, "TokenIsAppContainer yields a DWORD")
	assert.Zero(t, isAppC, "the test process is not in an AppContainer")

	assert.False(t, isAppContainerImpl(),
		"the test process is not sandboxed by MXC, so it must not report an app container")
}

// The false-positive guard against the real artefact, not a mock of it.
//
// A UAC-filtered token is what an ordinary non-elevated administrator runs with: deny-only
// BUILTIN\Administrators and a stripped privilege set, which is the same shape measured inside
// Codex CLI's Windows sandbox. A detector keyed on either of those would report every Windows
// desktop as sandboxed and turn the unconfined `direct` baseline row red.
//
// TokenLinkedToken hands back exactly that token on an elevated process, so the artefact is
// available in CI with no VM and no setup. IsTokenRestricted must still say false for it.
func TestIsRestrictedTokenNotFooledByUACFilteredToken(t *testing.T) {
	assert.False(t, isRestrictedTokenImpl(),
		"the test process itself is not sandboxed, so it must not report a restricted token")

	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &tok); err != nil {
		t.Fatalf("OpenProcessToken: %v", err)
	}
	defer func() { _ = tok.Close() }()

	elevated := tok.IsElevated()
	linked, err := tok.GetLinkedToken()
	if err != nil {
		// A standard user has no linked token. Nothing to prove here, and skipping says so
		// rather than passing vacuously.
		t.Skipf("no linked token to examine (elevated=%v): %v", elevated, err)
	}
	defer func() { _ = linked.Close() }()

	restricted, err := linked.IsRestricted()
	if err != nil {
		t.Fatalf("IsRestricted on the linked token: %v", err)
	}
	assert.False(t, restricted,
		"a UAC-filtered token has deny-only admin groups and stripped privileges just as the "+
			"Codex sandbox does, and must not be mistaken for a restricted token (this process "+
			"elevated=%v)", elevated)
}

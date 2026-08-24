//go:build windows
// +build windows

package tasks

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

// The badge's false case: with no restricted token and nothing else present, Windows must
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
// two pinned, isRestrictedToken is the only input left that can move the answer. That is what
// makes this a regression guard: it fails if anyone later wires a second Windows signal into
// the runtime chain without saying so.
func TestGetContainerRuntimeRestrictedTokenOnWindows(t *testing.T) {
	origToken, origRead, origExists := isRestrictedToken, readFile, fileExistsFunc
	t.Cleanup(func() { isRestrictedToken, readFile, fileExistsFunc = origToken, origRead, origExists })
	readFile = func(string) ([]byte, error) { return nil, fmt.Errorf("file not found") }
	fileExistsFunc = func(string) bool { return false }
	t.Setenv("container", "")

	for _, tt := range []struct {
		name       string
		restricted bool
		want       ContainerRuntime
	}{
		{"restricted token", true, RuntimeUnknown},
		{"unrestricted token", false, RuntimeNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isRestrictedToken = func() bool { return tt.restricted }
			assert.Equal(t, tt.want, GetContainerRuntime(0, 0),
				"container=%q — a non-empty value here promotes the answer to RuntimeUnknown",
				os.Getenv("container"))
		})
	}
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

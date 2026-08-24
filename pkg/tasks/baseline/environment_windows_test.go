//go:build windows
// +build windows

package tasks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

// The badge's false case, which is only deterministic here.
//
// On Windows every Linux detector is already a hard-false stub — isSeatbelt, probeForLandlock,
// isChroot and isProcSelfSetNoNewPrivs all come from the !darwin/!linux files, /proc reads fail,
// and /.dockerenv does not exist — so isRestrictedToken is the only input that can move the
// answer. That is what makes this the real regression guard: it fails if anyone later wires a
// second Windows signal into the runtime chain without saying so.
func TestGetContainerRuntimeRestrictedTokenOnWindows(t *testing.T) {
	orig := isRestrictedToken
	t.Cleanup(func() { isRestrictedToken = orig })

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
			assert.Equal(t, tt.want, GetContainerRuntime(0, 0))
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

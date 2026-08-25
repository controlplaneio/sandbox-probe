//go:build windows
// +build windows

package tasks

import (
	"unsafe"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
)

// tokenIsAppContainer is TokenIsAppContainer, TOKEN_INFORMATION_CLASS 29 in winnt.h.
//
// x/sys/windows does not declare it. It declares the same enum as an iota run and stops one
// short, at TokenLogonSid (28), so the value has to be written out here. The literal is the
// documented one; TestTokenIsAppContainerMatchesTheXSysEnum checks it against
// windows.TokenLogonSid+1, so the two sources have to agree and a future x/sys that inserts a
// class fails the test instead of silently querying the wrong thing.
const tokenIsAppContainer = 29

var isRestrictedToken = isRestrictedTokenImpl

// isRestrictedTokenImpl reports whether this process runs under a token produced by
// CreateRestrictedToken — the primitive behind Codex CLI's Windows sandbox
// (`[windows] sandbox = "unelevated"`), Chromium's renderer sandbox, `psexec -l` and
// others. It cannot say which of them built the token, which is why the caller reports it
// as a mechanism and never as a wrapper name.
//
// This is deliberately the ONLY signal. Measured on a Windows 11 host against Codex CLI
// 0.149.1, comparing the same user inside and outside the sandbox, the visible effects are:
// BUILTIN\Administrators demoted to "use for deny only", every privilege but
// SeChangeNotifyPrivilege stripped, and the integrity level unchanged at High.
//
// Both of the first two are also exactly what an ordinary non-elevated administrator's UAC
// split token looks like, so keying off them would report every unconfined Windows desktop
// as sandboxed and turn the `direct` baseline row red. IsTokenRestricted looks for
// restricting SIDs, which UAC filtering does not add: false for a filtered token, true only
// for a restricted one. Integrity is excluded on the measurement, not on principle — it is
// identical on both sides and carries no information.
//
// A real TOKEN_QUERY handle rather than the GetCurrentProcessToken pseudo-handle:
// pseudo-handle support is per-API and undocumented for this call, and this is not a bit to
// be unsure about. Errors swallow to false, matching isSeatbelt and isChroot — emitting no
// finding is recoverable, emitting a wrong one poisons every comparison the row appears in.
func isRestrictedTokenImpl() bool {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		log.Warn().Err(err).Msg("OpenProcessToken failed; reporting no restricted token")
		return false
	}
	defer func() { _ = tok.Close() }()

	restricted, err := tok.IsRestricted()
	if err != nil {
		log.Warn().Err(err).Msg("IsTokenRestricted failed; reporting no restricted token")
		return false
	}
	return restricted
}

var isAppContainer = isAppContainerImpl

// isAppContainerImpl reports whether this process runs inside an AppContainer — the primitive
// behind GitHub Copilot CLI's Windows sandbox, which is Microsoft Execution Containers (MXC)
// on its ProcessContainer backend.
//
// MXC's Windows backend calls Experimental_CreateProcessInSandbox in processmodel.dll, driven
// by a SandboxSpec whose app_container field its BaseContainerRunner always sets true. The
// spec's other controls are policy the caller chooses — UI restrictions via JOB_OBJECT_UILIMIT
// flags, the win32k system-call mitigation, least-privilege, integrity level — so each of them
// can be off in a real sandbox and none is safe to key off. The AppContainer is the one thing
// that is always there, so it is the one thing this reads.
//
// A mechanism and not a wrapper name, for the same reason isRestrictedTokenImpl is: an
// AppContainer token cannot say who created it. Store apps, Chromium's renderer and Defender
// Application Guard all produce one. Naming Copilot from this bit would repeat the
// `srt (linux)` mistake.
//
// The false-positive risk is lower than the restricted-token detector's, not higher. A
// restricted token is nearly the shape of an ordinary UAC split token, which is why that
// detector needed care. An AppContainer has no ambient analogue at all: a normal console
// process on a normal desktop is not in one, and only something that deliberately built an
// AppContainer can put the probe inside it.
//
// A real TOKEN_QUERY handle rather than the GetCurrentProcessToken pseudo-handle, matching
// isRestrictedTokenImpl. Errors swallow to false: on a Windows old enough to have no
// AppContainers the call fails with ERROR_INVALID_PARAMETER, and false is the true answer
// there anyway.
func isAppContainerImpl() bool {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		log.Warn().Err(err).Msg("OpenProcessToken failed; reporting no app container")
		return false
	}
	defer func() { _ = tok.Close() }()

	// TokenIsAppContainer yields a DWORD, non-zero inside an AppContainer.
	var isAppC uint32
	var retLen uint32
	err := windows.GetTokenInformation(tok, tokenIsAppContainer,
		(*byte)(unsafe.Pointer(&isAppC)), uint32(unsafe.Sizeof(isAppC)), &retLen)
	if err != nil {
		log.Warn().Err(err).Msg("TokenIsAppContainer query failed; reporting no app container")
		return false
	}
	return isAppC != 0
}

//go:build windows
// +build windows

package tasks

import (
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
)

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

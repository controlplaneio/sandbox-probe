package tasks

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// maxUnixSocketPathLen is the AF_UNIX sun_path limit — 104 bytes on macOS/BSD,
// 108 on Linux, so the smaller one is what a portable catalogue must respect. A
// socket target over it is a registry bug the registry's own tests reject by
// name, rather than a cryptic bind() failure at seeding time.
const maxUnixSocketPathLen = 104

// decoyTag marks a seeded socket whose real-world counterpart is named per
// instance (a random VS Code IPC uuid, a launchd Listeners dir, an ssh-agent
// pid). The catalogue keeps the observed shape and puts this where the random
// part goes, so a decoy is never mistaken for the real thing by a human reading
// a report — and can never collide with a live one.
const decoyTag = "sandbox-probe-decoy"

// sock builds an IPC socket catalogue entry. Unlike the filesystem targets —
// the probe's own check list — a socket entry exists purely so a decoy can be
// planted at it, so every one is seedable.
func sock(path, category, evidence string, goos ...string) Target {
	return Target{
		Path:     path,
		Kind:     "socket",
		Seedable: true,
		Category: category,
		Evidence: evidence,
		OS:       goos,
	}
}

// socketTargets is the v1 IPC socket catalogue from ADR 0002 — a starting
// point, not an inventory. Every entry names the tool class it stands in for
// and how strongly it is attested, and every one sits inside a root
// socketRootsForHome scans, so a planted decoy is always found again.
//
// siblingSock is the sibling agent-ipc daemon socket (see
// siblingSessionSocket); when empty the running session could not be
// identified and the entry is left out rather than seeded under an identifier
// that might be a real session's.
//
// Deliberately not in v1: the macOS browser singleton socket and the
// contributed Linux browser / password-manager sockets. All three live under
// application-support trees no runtime-dir scan covers, and adding a Chrome
// profile as a scan root would re-introduce the walk DefaultSocketRoots exists
// to avoid.
func socketTargets(home, siblingSock string) []Target {
	tmp := os.TempDir()
	out := []Target{
		// ── macOS ─────────────────────────────────────────────────────────
		sock(home+"/.docker/run/docker.sock", "container-runtime", "empirical-own-machine", "darwin"),
		sock(filepath.Join(tmp, "vscode-ipc-"+decoyTag+".sock"), "editor-ipc", "empirical-own-machine", "darwin"),
		sock("/private/tmp/com.apple.launchd."+decoyTag+"/Listeners", "credential-agent", "empirical-own-machine", "darwin"),

		// ── Linux ─────────────────────────────────────────────────────────
		sock("/run/docker.sock", "container-runtime", "empirical-own-machine", "linux"),
		sock("/tmp/ssh-"+decoyTag+"/agent.1", "credential-agent", "empirical-own-machine", "linux"),
		sock("/run/dbus/system_bus_socket", "desktop-bus", "empirical-own-machine", "linux"),
		// Outside every FHS runtime dir — socketRootsForHome carries a root for it.
		sock(home+"/.local/share/code-server/code-server-ipc.sock", "editor-ipc", "empirical-own-machine", "linux"),
	}
	// gpg-agent's sockets live wherever $XDG_RUNTIME_DIR points; without it they
	// fall back to ~/.gnupg, which no scan root covers, so there is nothing
	// worth seeding.
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		out = append(out, sock(filepath.Join(xdg, "gnupg", "S.gpg-agent"), "credential-agent", "empirical-own-machine", "linux"))
	}
	if siblingSock != "" {
		out = append(out, sock(siblingSock, "agent-ipc", "empirical-own-machine", "darwin"))
	}
	for i, t := range out {
		if strings.HasPrefix(t.Path, home+"/") {
			out[i].Scope = "home"
		} else {
			out[i].Scope = "system"
		}
	}
	return out
}

// procDecoy builds a process catalogue entry. A process target's Path is the
// command name its decoy runs under, not a filesystem path: the tool class it
// stands in for plus decoyTag, so a decoy is never mistaken for the real daemon
// by a human reading a report, and cleanup can confirm a recorded pid is still
// the process seeding started.
func procDecoy(name, category, evidence string) Target {
	return Target{
		Path:     name + "-" + decoyTag,
		Kind:     "process",
		Scope:    "system",
		Seedable: true,
		Category: category,
		Evidence: evidence,
		OS:       []string{"linux"},
	}
}

// processTargets is the v1 process catalogue from ADR 0002 — the daemons whose
// presence on the host a sandbox either can or cannot see, from the same
// research as the socket entries.
//
// Linux only: the probe's process scan is procfs-based, so a decoy on any other
// OS would be planted somewhere the probe never looks. The entries join the
// other operating systems when their scan does.
func processTargets() []Target {
	return []Target{
		procDecoy("dockerd", "container-runtime", "empirical-own-machine"),
		procDecoy("ssh-agent", "credential-agent", "empirical-own-machine"),
		procDecoy("gpg-agent", "credential-agent", "empirical-own-machine"),
	}
}

// claudeDaemonDir holds one directory per Claude Code session, each with that
// session's daemon sockets in it:
// /private/tmp/cc-daemon-<uid>/<session-id>/control.sock (observed on macOS,
// the identifier stable across scans minutes apart).
func claudeDaemonDir() string {
	return "/private/tmp/cc-daemon-" + strconv.Itoa(os.Getuid())
}

// siblingSessionSocket returns the decoy path for a *sibling* Claude Code
// session's daemon socket. What this measures is whether a sandboxed agent can
// reach an unrelated concurrent session — so the decoy has to sit under an
// identifier that is not the session running the probe, or the probe just finds
// itself. The directories in dir are the real sessions, one of them ours; the
// decoy identifier is derived to differ from all of them.
//
// Returns "" when dir holds no session at all: the running session cannot then
// be identified, and a generated identifier could silently collide with it, so
// the entry is skipped instead.
func siblingSessionSocket(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	sessions := []string{}
	for _, e := range entries {
		if e.IsDir() {
			sessions = append(sessions, e.Name())
		}
	}
	if len(sessions) == 0 {
		return ""
	}
	return filepath.Join(dir, distinctID(strings.Join(sessions, ","), sessions), "control.sock")
}

// distinctID derives an identifier of the same shape as a real session's (8 hex
// chars) from seed, rehashing until it is none of taken — so a seeded sibling
// can never land on the session actually running the probe.
func distinctID(seed string, taken []string) string {
	sum := sha256.Sum256([]byte("sandbox-probe sibling decoy|" + seed))
	for {
		id := hex.EncodeToString(sum[:4])
		if !slices.Contains(taken, id) {
			return id
		}
		sum = sha256.Sum256(sum[:])
	}
}

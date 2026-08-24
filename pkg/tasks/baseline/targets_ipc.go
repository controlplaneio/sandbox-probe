package tasks

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// pipePrefix is the Win32 path of the named-pipe object namespace. The
// namespace is machine-global (unlike \Sessions\<n>\BaseNamedObjects), so what
// a scan sees does not depend on the session it runs in. Defined here rather
// than beside the enumerator because the catalogue is cross-platform: the
// registry names Windows targets on every OS and lets the OS filter drop them.
const pipePrefix = `\\.\pipe\`

// errNoPipeNamespace is what the seeding half of the pipe seam reports off Windows. Declared
// here rather than in the !windows file because portable code needs to recognise it: seeding
// the reachability decoy must tell "there is no pipe namespace on this OS" apart from "the
// plant failed", and only the second is a skip worth counting.
var errNoPipeNamespace = errors.New("named pipes are Windows-only")

// ReachPipeName is the ONLY pipe this probe ever opens as a client, and it is deliberately
// NOT a catalogue entry.
//
// Opening a real service's pipe is not passive: it consumes a server instance, delivers a
// connection event, and can hang a badly written server. So reachability is measured only
// against a name no real service can hold.
//
// Why not the catalogue names, which are what an attacker would actually target. seedPipe
// plants a decoy at one of those only when it is free, so at SCAN time `\\.\pipe\docker_engine`
// is either our decoy or a live Docker Desktop — and the scan is a different process from the
// seeder, with no way to tell which. Making it tell would mean reading the seed record, which
// puts a SAFETY property behind a filesystem read the sandbox under test may block and whose
// TMPDIR may differ. That is exactly backwards. Whether the catalogue names are visible is
// already answered by named_pipe_detection; what reachability adds is whether an open succeeds,
// and that question can be asked of any pipe — so ask it of the one where the answer costs
// nobody anything.
//
// A name of this shape can only ever be ours: FILE_FLAG_FIRST_PIPE_INSTANCE means the seeder
// holds it or nobody does. It is fixed rather than per-run for the same reason the record is
// not consulted — seeder and scan share no state, and the only channel between them is that
// record. Two concurrent probe runs collide benignly: the second seeder's plant is skipped and
// the scan measures the first run's decoy, which is still the probe's own.
const ReachPipeName = pipePrefix + "sandbox-probe-reachability-decoy"

// reachControlPipeName is a name nothing ever serves: the calibration control, the same
// instrument as calibrate()'s known-free loopback port. It says what "absent" looks like on
// THIS host, so a not-found against the decoy is interpretable rather than merely
// disappointing. Per-pid, so two concurrent runs cannot make each other's control real.
func reachControlPipeName() string {
	return pipePrefix + "sandbox-probe-control-" + strconv.Itoa(os.Getpid())
}

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

// pipeDecoy builds a Windows named-pipe catalogue entry. Unlike a socket, a
// pipe only exists while a server holds it open, so its decoy is a live
// artifact on the belt-and-suspenders lifecycle — and unlike a process decoy it
// carries no decoyTag: the name is the whole point (nothing finds
// `docker_engine-sandbox-probe-decoy`), so cleanup identifies the server by the
// pid it spawned and that pid's creation time instead.
func pipeDecoy(name, category, evidence string) Target {
	return Target{
		Path:     pipePrefix + name,
		Kind:     "pipe",
		Scope:    "system", // the pipe namespace is machine-global, not per-user
		Seedable: true,
		Category: category,
		Evidence: evidence,
		OS:       []string{"windows"},
	}
}

// pipeTargets is the v1 Windows named-pipe catalogue from ADR 0002. Both names
// are the real ones — a pipe decoy under an invented name would stand in for
// nothing — so the soft-plant rule carries the safety here: a name a real
// service already serves is skipped, never shadowed.
func pipeTargets() []Target {
	return []Target{
		// OpenSSH ships with Windows 11; starting the service is what creates this.
		pipeDecoy("openssh-ssh-agent", "credential-agent", "empirical-own-machine"),
		// Docker Desktop's engine pipe. Documented, not observed: its installer is
		// GUI-bound and cannot be automated in the Session 0 research VM.
		pipeDecoy("docker_engine", "container-runtime", "documented-not-verified"),
	}
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

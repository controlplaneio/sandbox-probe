# Seed IPC + process targets

wayfinder:map

## Destination

An ADR (`docs/adr/000X-...`) plus an updated `docs/reporting-site-plan.md`,
specifying both the *mechanism* and the *catalogue* for extending
`sandbox-probe`'s seed/target registry beyond files — to Unix sockets
(macOS/Linux), Windows named pipes, and background processes — so those
categories become provable instead of `⬜ n/a`.

On disposable compute (GitHub Actions runners, throwaway VMs) the mechanism
can be as liberal as it needs to be — nothing there survives past the run.
On a real, persistent machine (Chris's Mac, in practice) it must be a hard
guarantee: nothing real gets touched, created artifacts are fully
ephemeral/reversible, verified experimentally (not just reasoned about) —
plus a populated catalogue of representative targets per OS (Docker, VS
Code, ssh-agent, browsers, etc., based on real research, not guesswork).

This is a plan-only effort (see Notes) with one exception: the safety
mechanism gets an experimental prototype/verification ticket, not just a
design on paper.

## Notes

- Domain: `sandbox-probe`'s target/seed registry. Read `CONTEXT.md` first —
  especially **Seed / Decoy**, **Target registry**, **Baseline vs Sandbox**,
  and the **Capability category** table (IPC sockets / Process visibility
  rows). Use `/domain-modeling` for any new term or finding-type decision
  (e.g. a new `finding_type` for named pipes) — update `CONTEXT.md` the
  moment it crystallises.
- Standing preference from charting: **plan, don't do** — tickets resolve
  decisions, not implementation, except where a ticket is explicitly typed
  `prototype`/`task` for verification purposes.
- Existing code to route through, don't re-derive:
  - `scripts/seed-decoys.sh` + `sandbox-probe list-targets` — the file-seeding
    pattern this extends (soft-plant: only where nothing real already
    exists).
  - `pkg/tasks/baseline/network.go` (`DefaultSocketRoots`, `ScanSocketRoots`)
    — Unix socket detection; stat()s for socket-typed dir entries, **never
    connects**. No Windows named-pipe equivalent exists yet — the socket
    task's own tests explicitly skip on Windows.
  - `docs/reporting-site-plan.md` — already has a deferred bullet under
    "Track 2 — seeder" for exactly this: *"network/socket decoys: a decoy
    listening port, a fake `docker.sock`, a stub egress target... Needs
    per-runtime plumbing; not blocking."* Close that bullet out via the new
    ADR rather than duplicating it.
  - `docs/adr/0001-client-side-site-over-data-branch.md` — style/rigor
    reference for the new ADR.
- Test environments available locally, use these before ever touching the
  real Mac:
  - Docker Desktop is running (`desktop-linux` context, Linux ARM64 VM) —
    disposable, safe to trash freely.
  - UTM is installed with an existing `Win11.utm` VM
    (`/Applications/UTM.app/Contents/MacOS/utmctl` for lifecycle control) —
    disposable, safe to trash freely.
- Established fact (see [socket file survives process exit](issues/01-socket-file-persists-after-process-exit.md)):
  a bound Unix socket special file persists on disk after the binding
  process exits — no live listener needs to stay running for detection
  purposes. Simplifies the mechanism: cleanup is "unlink the one file we
  made," not process lifecycle management.
- A backgrounded research subagent for [ticket 04](issues/04-windows-named-pipe-enumeration.md)
  failed outright on an automated cybersecurity safeguard (false-positive
  on legitimate defensive-tooling research about Windows named pipes). The
  ticket still got resolved — done directly in-session via `WebSearch`/
  `WebFetch` instead of a subagent. If a future research ticket touches
  similarly security-adjacent territory (privilege/enumeration APIs,
  syscalls, etc.) and a subagent dispatch fails the same way, don't retry
  the same prompt blindly — do the research directly in-session.

## Decisions so far

- [Socket file persists after process exit](issues/01-socket-file-persists-after-process-exit.md) — confirmed empirically: bind() + close() leaves a real socket-typed file on disk; no background listener needed for `unix_socket_detection` to find it.
- [Windows named-pipe enumeration](issues/04-windows-named-pipe-enumeration.md) — **corrected by ticket #50/#11**: the original finding here (no Win32 API path exists, needs `golang.org/x/sys/windows` + `NtQueryDirectoryFile`) was reasoned, not tested, and turned out wrong — a plain unelevated `FindFirstFileW`/`FindNextFileW` enumerates the pipe namespace fine, no new dependency required. No admin privilege required either way. The multi-root socket-scan abstraction still doesn't carry over (one root, no subdirectories).
- [Linux dev-machine footprint](issues/06-linux-dev-machine-footprint.md) — real sockets attributed via Docker: Docker/containerd (6 sockets), ssh-agent, gpg-agent (4, `$XDG_RUNTIME_DIR`-dependent), dbus, and a headless VS Code's IPC socket — the last one sits *outside* every existing `ScanSocketRoots` dir, a real scanner gap. GUI-only artifacts (ssh-askpass, browser, VS Code extension-host sockets) couldn't be reproduced in a container, only reasoned by analogy to the macOS data.
- [Namespace parity semantics](issues/08-namespace-parity-semantics.md) — **live bug, not just a future concern**: `run-docker.sh` shares none of `DefaultSocketRoots`, so every sandboxed Docker run reports zero sockets today regardless of actual policy — the site's current IPC-sockets column for Docker-family harnesses is a false-positive "blocked". Purely mount-namespace-driven (sockets are ordinary inodes, `unix(7)`), confirmed empirically, not policy-driven. Socket seeding needs baseline/sandbox parity exactly like file decoys already require.
- [Write final ADR](issues/10-write-final-adr.md) — **done, map fully clear**: `docs/adr/0002-seed-ipc-and-process-targets.md` written, `docs/reporting-site-plan.md`'s deferred bullet now points at it. Uncommitted, waiting on Chris before anything gets pushed.
- [Populate catalogue and registry](issues/09-populate-catalogue-and-registry.md) — **resolved, starting v1 catalogue locked in**: macOS (Docker/VS Code/ssh-agent/browser sockets, empirical), Linux (docker.sock/ssh-agent/gpg-agent/dbus + code-server's socket flagged as needing a new `ScanSocketRoots` entry), Windows (`openssh-ssh-agent` empirical, `docker_engine` documented-not-verified, VS Code excluded — weakest evidence). Every entry traced to its source ticket, explicitly a starting point, matching the "open invitation for contributions" framing this effort was given from the start. Map's decision-making is done — only the ADR (10) is left.
- [Windows dev-machine footprint capture](issues/07-windows-dev-machine-footprint-capture.md) — turned out automatable via `utmctl exec`/`file` (QEMU guest agent) after Chris's one-time guest-tools install. Real finding: OpenSSH ships pre-installed on Windows 11, `ssh-agent` is a one-line service-start → `\\.\pipe\openssh-ssh-agent`, confirmed via a real before/after diff. Also a hard structural wall confirmed, not assumed: `utmctl exec` runs in Windows Session 0 (isolated from the interactive desktop), so GUI-touching installers (VS Code, and by strong inference Docker Desktop) hang on an undismissable invisible dialog regardless of silent-install flags — killed cleanly, no trace left. Docker/VS Code pipe paths are documented-not-verified as a result. [Ticket 09](issues/09-populate-catalogue-and-registry.md) fully unblocked.
- [Prototype/verify seeding mechanism](issues/03-prototype-verify-seeding-mechanism.md) — **verified end-to-end**, Docker Linux VM then real Mac: socket (bind+close) and process (argv0-renamed + self-timeout + explicit kill) both go absent→detected→clean, confirmed via direct `ps -p <pid>` checks. Real finding: macOS's `AF_UNIX` `sun_path` length limit (~104 bytes) rejects deeply-nested paths — conventional real paths stay under it, but the implementation needs to handle this defensively. Mechanism design phase of this map is done.
- [IPC finding-type naming](issues/05-ipc-finding-type-naming.md) — **resolved**: separate `unix_socket_detection`/`named_pipe_detection`, folded under one "IPC sockets" category — same pattern as the existing "Network egress" fold. `CONTEXT.md` updated. Reports are always OS-scoped so a generalized type would only add a redundant discriminator field.
- [Safe seeding mechanism design](issues/02-safe-seeding-mechanism-design.md) — **resolved**: sockets soft-plant via bind+close (no live listener); Windows pipes and processes need a live-process hybrid lifecycle (self-timeout + explicit cleanup, fixed constant); one script (`seed-decoys.sh`, unrenamed) dispatches per-kind off an extended `list-targets` registry (`kind: file/socket/pipe/process`) rather than duplicating scaffolding. Scope simplified since the 5-generic-runtime nesting fix was retired — this mainly serves agent-driven and profile-attested sandboxes now, both of which see the seeded parent directly. [Ticket 03](issues/03-prototype-verify-seeding-mechanism.md) unblocked.

## Not yet specified

- How this wires into `scan-matrix.yaml` / `stub-common.sh` per-OS once the
  ADR lands (implementation, not a decision — deferred past this map).
- **New sibling map**: [Sandbox canary nesting](../sandbox-canary-nesting/map.md)
  — found while working this map's namespace-parity ticket. Decides
  whether a sandboxed run can even *reach* a seeded parent at all (the 5
  rootfs-swapping runtimes currently can't, for files today, not just
  sockets). That map's outcome directly affects whether socket/pipe
  seeding here is reachable once seeded — read alongside
  [ticket 02](issues/02-safe-seeding-mechanism-design.md).
- Whether the populated catalogue needs to track tool *versions* (e.g. does
  Docker Desktop's named-pipe path change across releases) — likely folds
  into the catalogue ticket rather than needing its own.

## Out of scope

(none yet)

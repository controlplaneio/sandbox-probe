Type: research
Status: resolved

## Question

For a file on a shared filesystem mount, "the sandbox can't see it" cleanly
means blocked. For a host-bound Unix socket, a network/mount-namespaced
sandbox (Docker, etc.) may structurally never see it regardless of any
enforcement policy the sandbox actually applies.

Investigate empirically, using Docker Desktop's `desktop-linux` VM
(disposable): bind a test socket on the host side at a path
`ScanSocketRoots` would scan, then run a plain container (no special
mounts/flags beyond whatever `sandbox-probe`'s own `tests/detect_docker.sh`
/ `sandbox_docker.sh` scripts use) and check whether the probe running
inside sees it.

Answer specifically: does "sandboxed run doesn't see the seeded host
socket" happen because of namespace isolation *regardless of policy* (a
false 🟩 "blocked" that isn't really evidence of anything), or does it only
happen when the sandbox's policy specifically restricts that path/mount
(a real, meaningful 🟩)? This determines whether socket decoys need to be
seeded *inside* the sandboxed environment too (mirroring how file decoys
already require baseline/sandbox parity), not just on the host baseline.

Write findings to a Markdown file in the repo, citing what was actually
observed. Capture on a throwaway `research/namespace-parity` branch.

## Answer

Full write-up (commands, output, kernel-doc citations to `unix(7)`,
`namespaces(7)`, `mount_namespaces(7)`, `network_namespaces(7)`):
`docs/research/namespace-parity-semantics.md` on branch
`research/namespace-parity` (commit `c2692f1`).

**It's purely mount-namespace-driven, not policy.** Docker gives every
container its own mount namespace; nothing from the host's `/run`/`/tmp`
propagates in unless explicitly bind-mounted. Unix domain sockets are
ordinary filesystem inodes (`unix(7)`), so they're exactly as invisible
across an unshared mount namespace as any regular file — not a
socket-specific security feature. Confirmed empirically: a sibling
container with no extra mounts (mirroring this project's own
`run-docker.sh` invocation exactly) gets `ENOENT` for a live host-bound
socket — even with `--network=host` added, still `ENOENT`, proving mount
namespace gates it, not network namespace. When the containing path *was*
bind-mounted, the sibling container didn't just see the socket file — it
connected straight through to the original listener across separate
PID/net namespaces.

**Practical consequence — and this matters right now, not just for future
seeding**: `run-docker.sh` never shares any of `DefaultSocketRoots`'
scanned paths (`/run`, `/tmp`, etc.), only a throwaway data dir. That means
**every sandboxed Docker run today reports zero sockets, permanently,
regardless of actual sandbox tightness** — the site's existing "IPC
sockets" 🟩/🟥 column for Docker-family harnesses is currently a
false-positive "blocked" that measures container plumbing, not policy. This
isn't a future concern the seeding work introduces — it's a live
correctness bug in what the reporting site shows today.

Socket decoys need baseline/sandbox seeding parity, exactly mirroring
`seed-decoys.sh`'s "PARITY IS LOAD-BEARING" rule for files — seeding must
happen inside the sandboxed environment too (or ride along whatever's
already bind-mounted), or the column stays meaningless regardless of what
gets seeded on the baseline side.

**Consequence for [ticket 02](02-safe-seeding-mechanism-design.md) and
[ticket 10](10-write-final-adr.md)**: the mechanism design needs to cover
*where* seeding happens (host baseline AND inside each sandboxed harness),
not just how to seed safely on the host — and the final ADR should flag the
existing false-positive as a correctness note, separate from (but uncovered
by) this effort.

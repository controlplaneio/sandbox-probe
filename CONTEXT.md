# Context

Ubiquitous language for `sandbox-probe` and the reporting site built on top of it.

## Glossary

### Probe
The single static Go binary that runs inside a sandbox and records what the
kernel let it do. One invocation is a **scan**.

### Scan
One execution of the probe. Produces exactly one **Report**.

### Report
The JSON document a scan emits: probe build metadata, run tags, and a list of
**Findings**. The unit the site ingests.

### Finding
One thing the probe was *able* to do, identified by a stable `findingType`
(e.g. `sensitive_readable_paths`, `external_host_connectivity`,
`sandbox_detection`). **Presence of a finding means the sandbox did not block
that capability.** Absence means it was blocked. This inversion is the whole
game: fewer findings = tighter sandbox.

### Harness
The thing whose sandbox is under test — an AI coding agent (Claude Code, Codex,
Gemini CLI, opencode, goose, pi, gptme, cline, trae) or a raw sandbox runtime
(docker, podman, nono, bwrap, gvisor, firejail, nspawn, srt). A harness may
appear in confined and unconfined variants (e.g. `claude` vs `claude-sandbox`);
each variant is a distinct harness identity for reporting.

### Baseline vs Sandbox
The methodology is comparison, not absolute measurement. The **baseline** is the
probe run unconfined on the host; the **sandbox** run is inside the harness. The
sandbox boundary is everything present in the baseline but absent under the
sandbox.

### Run
One Report for one harness identity at one point in time.

### Time-series identity
The stable key grouping runs into one trend line: the tuple `(os, harness)`
(e.g. `macos-claude-sandbox`), read from tags/filename so new harnesses join
with no code change. Harness/probe/kernel versions are *not* part of identity —
they move along the line.

### Fingerprint / Point
A plotted point is a distinct **configuration fingerprint**, not a calendar
date: `harness version + probe commit + kernel release + OS release`. Runs
sharing the entire fingerprint collapse to one point (redundant re-runs dedup);
any component changing starts a new point; latest run wins on a collision.
Points order along the axis by first-seen timestamp, so the axis is a *sequence
of distinct configurations*, not wall-clock time.

### Time series
The ordered sequence of points for one identity — the substrate for tracking
degradations (a blocked capability becoming reachable) and improvements (the
reverse).

### Regression / Degradation
A capability that was blocked in an earlier snapshot becoming reachable in a
later one — a finding appearing where there was none. Typically caused by a
version change in the harness or its supporting technology. The inverse
(a finding disappearing) is an **improvement**.

### Target registry
The probe's own list of things it checks (sensitive paths, and later network /
socket targets). The probe is the single source of truth and exposes it
(`list-targets`) so the seeder cannot drift from what is actually probed. The
listing is OS-scoped: a target applicable only to another operating system is
not emitted, so the seeder never attempts a Windows pipe on Linux.

### Kind
*How* a target is seeded — one of `file`, `dir`, `socket`, `pipe`, `process`.
The seeder dispatches on it.

### Category
*Why* an IPC target (`socket` / `pipe` / `process`) is on the list: the
real-world tool class it stands in for — one of `container-runtime`,
`credential-agent`, `editor-ipc`, `agent-ipc`, `chat-client`, `browser`,
`password-manager`, `desktop-bus`. Filesystem targets carry none: they are the
probe's own check list, not a tool catalogue.

### Evidence
How strongly an IPC target is attested — one of `empirical-own-machine`,
`empirical-contributed` (names its source), `documented-not-verified`,
`reasoned-by-analogy`. Keeps a reasoned-by-analogy path from passing as an
observed one when the catalogue is extended by contribution.

### Seed / Decoy
A harmless stand-in planted at a real canonical path (a fake `~/.aws/credentials`,
dummy SSH key, …) so a capability becomes *achievable* and a sandbox blocking it
becomes provable rather than ⬜ n/a. Seeding is **soft**: a decoy is written only
where nothing already exists, so a real secret is never overwritten. The seed
must be planted **identically in the baseline and every sandbox run** — parity is
what makes the diff mean "the sandbox blocked it" rather than "the file was
absent." A `socket` decoy is a real Unix socket bound and closed at a catalogue
path; nothing listens on it, because detection only stat()s. A `process` decoy
is a live process the seeder started itself under a distinctive command name —
never an adopted one — so the process scan has something of the host's to find.
A `pipe` decoy is a Windows named pipe served under its real catalogue name by a
probe process the seeder spawned; a pipe exists only while a server holds it
open, so the server is the decoy, and a name a real service already serves is
skipped rather than shadowed.

### Belt and suspenders
The lifecycle of a decoy that has to stay alive during the scan (a `process` or
a Windows `pipe`). The **belt** is a fixed self-timeout, comfortably
longer than a scan, so the artifact dies on its own even if nothing cleans up
after a crashed run. The **suspenders** are the cleanup pass, which is the
normal path and never waits the timeout out.

### Seed record
What one seeding pass created, written down so the cleanup pass after the scan
removes exactly that and nothing else. Cleanup is idempotent and tolerates a
record left by a crashed run: an artifact already gone, or one that is no longer
the artifact that was planted, is left alone. For a live artifact the record
carries an identity as well as a location — the process id and the command name
it was seeded under, or for a pipe server the process id and its creation time —
and nothing is signalled or terminated unless the pid still matches, so a reused
pid can never cost an unrelated process.

### Sibling session
An unrelated, concurrent agent session on the same host. The agent-ipc decoy is
seeded under a session identifier deliberately *not* the one running the probe,
so a finding measures cross-instance reach rather than the probe finding itself;
where the running session cannot be identified, the entry is skipped rather than
seeded under an identifier that might be a real session's.

### Cell states (baseline-normalized)
Every capability cell is read relative to the same-OS unconfined baseline:
🟥 **leaked** (baseline could, this harness still can), 🟩 **blocked** (baseline
could, this harness cannot), ⬜ **n/a** (baseline could not either — nothing to
prove).

### Capability category
The 8 leak categories that form the matrix columns and the 0–8 exposure count.
Seven are baseline-normalized (a door the baseline had); the eighth (Privileged
execution) is absolute.

| Category | finding_types | Rule |
|---|---|---|
| Filesystem read | `sensitive_readable_paths` | baseline-diff |
| Filesystem write | `writeable_paths` | baseline-diff |
| Network egress | `external_host_dns_resolution`, `external_host_connectivity` (folded) | baseline-diff |
| Local services | `tcp_ports_open`, `udp_ports_open` | baseline-diff |
| IPC sockets | `unix_socket_detection`, `named_pipe_detection` (folded) | baseline-diff |
| Process visibility | `process_detection`, `parent_process_detection` | baseline-diff |
| Host mounts | `mounted_volumes_detections` | baseline-diff |
| Privileged execution | `user_context_detection` | absolute: euid 0 = 🟥 |

Context (not counted): `sandbox_detection` (enforcement badge), `hostname_detection`,
`environment_detection` (identity/kernel, feeds fingerprint), `proxy_detection`
(drill-down), `env_secret_detection` (credentials the run was handed — what the
process already had, not a door it opened, so it is context and the scale stays 0–8). Unmapped future finding types → an **Other** column, uncounted.

### Exposure
The headline scalar the eye tracks over time: the count of leaked (🟥) capability
categories for an identity at a point (0–8). Rising = degrading sandbox, falling =
improving. The y-axis of the exposure-over-time chart.

### Flip / Flip-log
A **flip** is a capability changing state between two consecutive points of one
identity (🟩→🟥 degradation, 🟥→🟩 improvement), attributed to the fingerprint
component that moved (harness / probe / kernel / OS). The **flip-log** is the
chronological list of flips — the actionable text beside the charts.

### Enforcement badge vs mechanism
`sandbox_detection` carries two different kinds of claim, and a new detector
must know which one it is contributing to:
- The **enforcement badge** — the wrapper name (`bubblewrap`, `docker`,
  `firejail`, …) — is an *inferred* best guess at the tool, built from
  ancestry, markers and, as a last resort, a restricted user namespace's ID
  map. Treat it as a hypothesis, not an attested fact.
- A **mechanism** (`seccomp-filter`, `no-new-privs`, `landlock`,
  `user-namespace`, …) is *kernel-attested*: read directly off a kernel
  interface (`/proc/self/status`, the uid_map), true regardless of whether the
  wrapper name resolved. Mechanisms are emitted alongside the badge, never
  folded into it.

The user-namespace rule (a non-identity uid_map) is the **last resort** in the
wrapper-name chain, tried only after every more specific runtime detector has
had its chance to claim the run — a new detector belongs *above* it, not
below.

### Tags
`key=value` strings on a report's metadata carrying the run's context: the
harness, its version (`claude=2.1.202`), sandbox mode, runner OS. Versions are
attributes that move *along* a time series, not part of a harness's identity.

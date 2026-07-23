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
(`list-targets`) so the seeder cannot drift from what is actually probed.

### Seed / Decoy
A harmless stand-in planted at a real canonical path (a fake `~/.aws/credentials`,
dummy SSH key, …) so a capability becomes *achievable* and a sandbox blocking it
becomes provable rather than ⬜ n/a. Seeding is **soft**: a decoy is written only
where nothing already exists, so a real secret is never overwritten. The seed
must be planted **identically in the baseline and every sandbox run** — parity is
what makes the diff mean "the sandbox blocked it" rather than "the file was
absent."

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
| IPC sockets | `unix_socket_detection` | baseline-diff |
| Process visibility | `process_detection`, `parent_process_detection` | baseline-diff |
| Host mounts | `mounted_volumes_detections` | baseline-diff |
| Privileged execution | `user_context_detection` | absolute: euid 0 = 🟥 |

Context (not counted): `sandbox_detection` (enforcement badge), `hostname_detection`,
`environment_detection` (identity/kernel, feeds fingerprint), `proxy_detection`
(drill-down). Unmapped future finding types → an **Other** column, uncounted.

### Exposure
The headline scalar the eye tracks over time: the count of leaked (🟥) capability
categories for an identity at a point (0–8). Rising = degrading sandbox, falling =
improving. The y-axis of the exposure-over-time chart.

### Flip / Flip-log
A **flip** is a capability changing state between two consecutive points of one
identity (🟩→🟥 degradation, 🟥→🟩 improvement), attributed to the fingerprint
component that moved (harness / probe / kernel / OS). The **flip-log** is the
chronological list of flips — the actionable text beside the charts.

### Tags
`key=value` strings on a report's metadata carrying the run's context: the
harness, its version (`claude=2.1.202`), sandbox mode, runner OS. Versions are
attributes that move *along* a time series, not part of a harness's identity.

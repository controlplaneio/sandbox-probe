# Context

Ubiquitous language for `sandbox-probe` — the binary, its registry, and the
reports it emits.

The comparison-side vocabulary (harness, baseline vs sandbox, identity,
fingerprint, cell states, capability category, exposure, flip) lives with the
comparison layer, in
[`sandbox-probe-reports`](https://github.com/chrisns/sandbox-probe-reports)'s
`CONTEXT.md`. That is the authoritative definition of those terms; this file
must not fork them.

## Glossary

### Probe
The single static Go binary that runs inside a sandbox and records what the
kernel let it do. One invocation is a **scan**.

### Scan
One execution of the probe. Produces exactly one **Report**.

### Report
The JSON document a scan emits: probe build metadata, run tags, and a list of
**Findings**.

### Finding
One thing the probe was *able* to do, identified by a stable `findingType`
(e.g. `sensitive_readable_paths`, `external_host_connectivity`,
`sandbox_detection`). **Presence of a finding means the sandbox did not block
that capability.** Absence means it was blocked. This inversion is the whole
game: fewer findings = tighter sandbox.

### Tags
`key=value` strings on a report's metadata carrying the run's context — what
ran the probe, its version, the runner OS. Supplied with `--tags`; the probe
records them verbatim and attaches no meaning of its own.

### Target registry
The probe's own list of things it checks (sensitive paths, sockets, pipes,
processes). The probe is the single source of truth and exposes it
(`list-targets`) so anything seeding decoys cannot drift from what is actually
probed. The listing is OS-scoped: a target applicable only to another operating
system is not emitted, so a seeder never attempts a Windows pipe on Linux.
`list-targets` and the report JSON are the probe's whole external interface.

### Kind
*How* a target is seeded — one of `file`, `dir`, `socket`, `pipe`, `process`.
A seeder dispatches on it.

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
becomes provable rather than "nothing was there to find". Seeding is **soft**: a decoy is written only
where nothing already exists, so a real secret is never overwritten. A `socket` decoy is a real Unix socket bound and closed at a registry
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

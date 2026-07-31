# 2. Extend the seed/target registry to IPC sockets, named pipes, and processes

Date: 2026-07-31

## Status

Accepted

## Context

`sandbox-probe`'s baseline-normalization methodology (ADR 0001) only proves
a capability "blocked" rather than "n/a" if the baseline could achieve it
too — which for files is handled by `seed-decoys.sh` soft-planting decoys
at `list-targets`'s registry paths. Three capability categories have no
equivalent: `unix_socket_detection`, `process_detection`/
`parent_process_detection`, and (once added) Windows named pipes all stay
`⬜ n/a` on a bare CI runner, because nothing seeds them. This was already
flagged as deferred, not forgotten, in `docs/reporting-site-plan.md`'s
"Track 2 — seeder" section: *"(Fast-follow, deferred) network/socket
decoys... Needs per-runtime plumbing; not blocking."* This ADR is that
fast-follow.

Four forces shaped the decision:

- **The safety bar differs by compute.** Disposable compute (GitHub Actions
  runners, throwaway VMs) can be seeded liberally — nothing survives past
  the run. A real, persistent machine (a developer's own laptop, where this
  probe is also meant to be run standalone) needs a hard guarantee: nothing
  real gets touched, ever.
- **Sockets and live artifacts (processes, Windows pipes) are not the same
  kind of thing.** A bound Unix socket special file persists on disk after
  its creating process exits — confirmed empirically (`bind()`+`close()`
  leaves a real socket-typed file; `unix_socket_detection` only `stat()`s
  for socket-typed entries, it never connects). A process, and a Windows
  named pipe (which only exists while something is listening on it), both
  need something alive during the scan.
- **Comparing what we configure ourselves is circular.** A sibling
  investigation (`sandbox-canary-nesting`, spawned directly from this
  work) found that the 5 generic sandbox-family runtimes
  (docker/podman/bwrap/nspawn/gvisor with no agent driving them) can't be
  meaningfully compared at all — any sharing flags a test harness adds
  itself are its own choice, not a vendor's. This narrowed what "seeding
  needs to be reachable from" actually means: primarily the agent-driven
  sandboxes (`claude-sandbox`/`codex-sandbox`, confirmed to run in the same
  process tree as the seeded parent — no separate reachability fix needed)
  and profile-attested sandboxes, not the 5 retired runtimes.
- **The catalogue itself is real research, not invention.** Populating
  "what does a real developer machine's IPC footprint look like" requires
  actual data — gathered here from a live scan of a real Mac, a Docker
  Linux VM running representative dev tooling, and a real Windows 11 VM —
  not guesswork.

## Decision

**Mechanism** — extends `seed-decoys.sh` (name unchanged) rather than a
parallel script, dispatching per target `kind`:

- `list-targets`'s registry gains a `kind` field: `file` (existing) /
  `socket` / `pipe` / `process`. One script, one soft-plant/parity
  invocation point in `scan-matrix.yaml`, not several that could drift.
- **Sockets**: soft-plant via `bind()` + `close()` at the conventional
  path — no live listener, cleanup is unlinking the one file created.
  Skipped if something's already there (soft, same rule as file decoys) —
  which also means the decoy only ever matters where the real tool is
  absent; where it's running for real, the real socket already makes the
  finding provable.
- **Processes and Windows named pipes** (no fire-and-forget equivalent —
  a named pipe only exists while something's listening): a hybrid
  lifecycle. Belt: a generous self-timeout so the seeded process dies on
  its own regardless of what else happens. Suspenders: explicit cleanup
  by recorded PID right after the scan, not relying on the timeout as the
  normal path.
- **Real constraint found empirically, not assumed**: macOS (and Unix
  generally) enforces an `AF_UNIX` `sun_path` length limit (~104 bytes).
  Conventional real-world paths stay under it; the implementation needs to
  handle or clearly surface a violation rather than fail cryptically.

**New finding type**: `named_pipe_detection`, kept separate from
`unix_socket_detection` rather than generalized into one cross-platform
type — a report is always OS-scoped (never both platforms at once, already
knowable from that report's own `environment_detection`), so a
discriminator field would be redundant. Both fold under the existing "IPC
sockets" category in `CONTEXT.md`'s Capability category table, the same
pattern already used for "Network egress" folding two finding types.
`CONTEXT.md` updated as part of this decision; `README.md`'s detection
table and `site/app.js`'s `FT2CAT` mapping are implementation follow-ups,
not done as part of this design.

**Catalogue taxonomy** — each entry carries `category` (the real-world
tool class: `container-runtime`, `credential-agent`, `editor-ipc`,
`agent-ipc`, `chat-client`, `browser`, `password-manager`...) and
`evidence` (`empirical-own-machine` / `empirical-contributed` /
`documented-not-verified` / `reasoned-by-analogy`), on top of the `kind`
field from the mechanism decision above. `kind` is *how* it gets seeded;
this taxonomy is *why* it's in the list — the point is a registry legible
and PR-able by outside contributors, not a flat path dump only the
original author can safely extend. `evidence-contributed` entries name
their source.

**Catalogue (v1, not exhaustive)**:

| OS | Category | Entry | Evidence |
|---|---|---|---|
| macOS | container-runtime | Docker Desktop socket | empirical-own-machine (live Mac scan) |
| macOS | editor-ipc | VS Code IPC socket | empirical-own-machine |
| macOS | credential-agent | ssh-agent/askpass socket | empirical-own-machine |
| macOS | browser | one browser singleton socket | empirical-own-machine |
| macOS | agent-ipc | **a sibling Claude Code instance's daemon socket** — see below | empirical-own-machine |
| Linux | container-runtime | `docker.sock` | empirical-own-machine (Docker container research) |
| Linux | credential-agent | `ssh-agent` socket | empirical-own-machine |
| Linux | credential-agent | one `gpg-agent` socket | empirical-own-machine (one of 4 observed variants) |
| Linux | desktop-bus | dbus system bus socket | empirical-own-machine |
| Linux | editor-ipc | code-server IPC socket | empirical-own-machine, but **inert until `ScanSocketRoots` gains a new root** — it sits outside every path currently scanned |
| Linux | browser | Brave singleton/mDNS sockets | **empirical-contributed** — a contributor's real desktop inventory (439 sockets observed), supersedes the earlier container-research exclusion below |
| Linux | password-manager | 1Password socket | **empirical-contributed** — the same contributed inventory (18 sockets), not previously considered at all |
| Windows | credential-agent | `\\.\pipe\openssh-ssh-agent` | empirical-own-machine (real before/after service-start diff) |
| Windows | container-runtime | `\\.\pipe\docker_engine` | documented-not-verified (blocked — see Consequences) |
| Processes | — | Docker/ssh-agent/gpg-agent process names | reuses the same research, same mechanism as sockets |

**Claude Code sibling-instance socket, reasoning corrected during review**:
the first draft of this catalogue excluded Claude Code's own daemon
socket outright as "circular — decoying the tool running the probe."
That's true only for the *current* session's own socket, which is a
different thing from the actual security question: **can a sandboxed
agent reach an unrelated, concurrent Claude Code session on the same
host?** Multiple simultaneous instances (a sandboxed session alongside an
unsandboxed one, or two unrelated sandboxed sessions) is a real, likely
scenario, and cross-instance reachability is a genuine lateral-movement
concern specific to exactly what this project measures. Confirmed
empirically, twice, in the same conversation: the real path shape is
`/private/tmp/cc-daemon-<uid>/<session-hash>/{control.sock,rv/*.sock,spare/*.sock}`,
and the `<session-hash>` (`830dcffe` in this session) stayed identical
across two live scans run minutes apart — a stable per-session identifier,
not per-scan noise. **The decoy must therefore use a different, fake
session-hash directory than whatever session is actually running the
probe** — seeding under the real running session's own hash would
reintroduce the exact circularity being corrected. This is a mechanism
nuance for whoever implements ticket 02's design, not just a catalogue
addition: the seeder needs to know the *real* running session's own hash
(to avoid it) when generating the fake sibling one.

**Still excluded from v1, explicitly**: GUI-only artifacts never actually
observed on Linux (ssh-askpass specifically — it's a transient,
per-auth-prompt socket, and neither the container research nor the
contributed static inventory snapshot would have caught one mid-prompt;
still reasoned-by-analogy, not real evidence, even after this round), and
VS Code's Windows IPC pipe (weakest evidence, pattern-only naming). This
catalogue is a starting point by design — an open invitation for
contributions, matching how this was framed to the wider community from
the start, and now demonstrated directly by the contribution above.

## Consequences

- **Correction (2026-07-31, ticket #50)**: the bullet below, as originally
  written, claimed no Win32 API exposes pipe enumeration at all and required
  a new `golang.org/x/sys/windows` dependency plus raw `NtQueryDirectoryFile`
  handling. That claim was **wrong** — it was reasoned from Sysinternals'
  `PipeList` documentation, not tested. Ticket #11 tested it directly: on a
  real Windows 11 VM, as a genuinely standard, unelevated local user (token
  carries `BUILTIN\Users`, no Administrators entry — `ADMIN=False`), a plain
  Win32 `FindFirstFileW("\\.\pipe\*")`/`FindNextFileW` loop enumerated the
  pipe namespace and returned 57 pipe names, terminating with
  `GetLastError()=18` (`ERROR_NO_MORE_FILES`) — normal end of enumeration,
  not an access failure. That is the API pair Go's `syscall.FindFirstFile`/
  `FindNextFile` already wraps. **No new dependency, no cgo, no ntdll
  import, and no NTSTATUS handling are required for pipe enumeration.** The
  struck-through text immediately below is kept, not deleted, so the
  original (incorrect) reasoning stays legible: it was inferred from
  Sysinternals' Win32-API silence on the topic, without an unelevated
  empirical test on that specific API pair, and that inference turned out to
  be wrong. Full evidence: ticket #11's first comment. This correction
  changes only this bullet — the finding-type split and the catalogue above
  are unaffected.
- ~~Windows named-pipe detection is genuinely new Go code, not a thin
  platform shim: no Win32 API exposes pipe enumeration at all (confirmed
  against Microsoft's own Sysinternals docs); the real mechanism is the
  native `NtQueryDirectoryFile`, reachable via a new
  `golang.org/x/sys/windows` dependency (not currently in `go.mod`) —
  implementation work this ADR scopes but does not perform.~~ *(superseded,
  see correction above)*
- `pkg/tasks/baseline`'s mount enumerator (`mounted_volumes_detections`)
  was found, incidentally, not to surface a real gVisor bind mount even
  though the file behind it was reachable — a separate probe bug, tracked
  but out of scope here.
- The probe already has dead code purpose-built for the `env_credentials`
  category nono profiles restrict most tightly (`models.EnvFinding`,
  `detectSensitiveEnvVars()`) that's never wired to a `finding_type` —
  found while mapping nono's profile schema to probe findings in the
  sibling `profile-attestation` effort, worth fixing independent of this
  ADR.
- Reachability from inside a sandbox is **not** solved by this ADR alone —
  it depends on the sandbox actually being nested in the seeded parent,
  which `sandbox-canary-nesting` found is already true for the
  agent-driven harnesses (no extra work needed there) but explicitly not
  true for the 5 generic runtimes (retired from this comparison
  entirely, not fixed).
- Full research backing lives on local branches, not yet merged:
  `research/windows-named-pipes`, `research/linux-dev-footprint`,
  `research/namespace-parity`, `research/windows-dev-machine-footprint`,
  each with cited primary sources and raw command output. The pipe
  enumeration branch's central finding is **superseded** by the correction
  above — the doc itself is kept as-is (not rewritten) with the branch
  marked corrected, per ticket #50, rather than silently edited.
- This closes the deferred "network/socket decoys" bullet in
  `docs/reporting-site-plan.md`'s "Track 2 — seeder" section — updated to
  point here rather than restate the plan.
- A contributor sent over a real Linux desktop inventory (script + output,
  reviewed and confirmed read-only/safe before use) plus a proposed
  socket-owner-attribution methodology (an evidence ladder: direct
  listener → service manager → container controller → inherited →
  unknown, with `attribution_level`/`attribution_reason` fields). The
  inventory data is folded into the catalogue above. The attribution
  methodology itself is **deliberately not adopted** here — it answers "who
  really owns this live socket during a real scan," a different, larger
  question than "why is this entry in our seed list," and would need its
  own design work if pursued; noted for a future effort, not scoped into
  this one. Full write-up:
  `docs/research/linux-desktop-inventory-contribution.md` on branch
  `research/linux-desktop-inventory` (see that doc for the taxonomy
  detail and the full evidence-ladder proposal, preserved rather than
  discarded).

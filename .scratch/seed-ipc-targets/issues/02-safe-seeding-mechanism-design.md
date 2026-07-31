Type: grilling
Status: resolved

## Question

What is the actual seeding mechanism for sockets, named pipes, and
processes — on a real, persistent machine, where "never touch anything
real" is a hard guarantee, not a best-effort?

Candidates to work through with Chris (not exhaustive — surface more during
the session):

- **Path choice**: seed at the exact conventional path a real tool would use
  (e.g. `~/.docker/run/docker.sock`-shaped), relying on the existing "soft"
  rule (skip if already present) — same philosophy as `seed-decoys.sh` for
  files. Given [the socket-persistence fact](01-socket-file-persists-after-process-exit.md),
  this is bind()+close()+defer-unlink, no live process.
  - Consequence worth surfacing to Chris: on a machine where the real tool
    (e.g. Docker Desktop) is actually running, the soft rule means the
    decoy is skipped — the real socket already makes the finding provable.
    The decoy only ever matters where the real tool is absent. Narrows the
    real risk band, but doesn't eliminate it (transient/session-dependent
    sockets like `ssh-askpass` or an IDE's IPC socket may be legitimately
    absent on Chris's Mac in some sessions).
- **Process seeding**: unlike sockets, a "seeded process" needs to actually
  exist as a process for `process_detection`/`parent_process_detection` to
  find it — this is a live-process lifecycle question (start before scan,
  ensure teardown even on crash/interrupt) in a way sockets are not. Decide
  the mechanism (a managed subprocess with a trap/defer, a short-lived
  helper, something else) and how it's guaranteed not to leak past the scan
  on a real machine.
- **Where the seed step lives**: extend `scripts/seed-decoys.sh` in place,
  or a new sibling script (`seed-ipc.sh`?) invoked alongside it — decide
  based on how cleanly the "soft" + cleanup semantics fit the existing
  script's shape.
- **Verification requirement**: whatever's decided here must be provably
  safe before it ever touches Chris's real Mac — that's
  [the prototype ticket](03-prototype-verify-seeding-mechanism.md), blocked
  on this one.

## Answer

Resolved via grilling. Scope note first: this mechanism now mainly serves
the agent-driven sandboxes (`claude-sandbox`/`codex-sandbox`, confirmed via
[sandbox-canary-nesting tickets 11/12](../sandbox-canary-nesting/issues/11-claude-sandbox-nesting.md)
to run in the *same process tree* as the parent — a host-side seed is
directly visible to them, no separate nesting fix needed) and the
profile-attested cases from
[profile-attestation](../profile-attestation/map.md). The 5 generic
runtimes' mount-flag fix was retired entirely (circularity finding), so
this ticket is simpler than originally scoped — no need to design around
making seeded sockets reachable through docker/podman/bwrap/nspawn/gvisor
specifically.

Three sub-decisions:

1. **Sockets vs. pipes are asymmetric, not parallel.** Sockets: soft-plant
   at the exact conventional path, `bind()` + `close()` — no live listener
   needed, per [ticket 01](01-socket-file-persists-after-process-exit.md)'s
   empirical fact. Windows named pipes have no equivalent — a pipe
   instance only exists while something's listening (`CreateNamedPipe`),
   so seeding a pipe needs the same live-process lifecycle as process
   seeding, not the fire-and-forget socket path.
2. **Live artifacts (processes, Windows pipes) get a hybrid lifecycle**:
   belt — a generous self-timeout (a few minutes, comfortably longer than
   a scan) so the seeded process/pipe-listener dies on its own regardless
   of what else happens; suspenders — explicit cleanup by recorded PID
   right after the scan completes, so the normal path doesn't rely on
   waiting out the timeout. Fixed constant, not configurable (no reason
   given to need it tunable). Mirrors the caution the nono `--dry-run`
   incident already forced this session — never trust a single cleanup
   path on a real, persistent machine.
3. **One script, extended registry.** `list-targets` gains a `kind` field
   (`file`/`socket`/`pipe`/`process`); `seed-decoys.sh` (name unchanged —
   "decoys" still applies broadly, no reason to churn the rename across
   `scan-matrix.yaml`/docs) dispatches per-kind internally rather than
   duplicating the read-registry/soft-plant scaffolding across separate
   scripts. Keeps "parity is load-bearing" as one invocation point, not
   several places that could drift.

[Ticket 03 (prototype/verify)](03-prototype-verify-seeding-mechanism.md)
is now unblocked.

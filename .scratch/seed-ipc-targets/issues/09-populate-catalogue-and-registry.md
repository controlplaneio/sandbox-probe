Type: grilling
Status: resolved

Blocked by: 05, 06, 07

## Question

With the finding-type shape decided, and real Linux and Windows footprint
data in hand (alongside the macOS data already gathered during charting),
decide the actual populated catalogue: which specific sockets, named
pipes, and processes get seeded, per OS — and where that list lives in the
codebase (extend `list-targets`'s JSON shape to non-file target types, or
a parallel registry) so `seed-decoys.sh` (or its extension) stays the
single source of truth the way file targets already are.

This is the ADR's concrete deliverable content, not just its framework —
per Chris's call during charting, the catalogue itself (not just the
mechanism) is in scope for this map.

## Answer

Registry shape was already settled in
[ticket 02](02-safe-seeding-mechanism-design.md) (extend `list-targets`
with a `kind` field: `file`/`socket`/`pipe`/`process`) — this ticket
resolves the concrete v1 catalogue, every entry traced to the ticket that
found it, none invented here.

**macOS** (empirical, gathered live during charting): Docker Desktop
socket, VS Code IPC socket, an ssh-agent/askpass socket, one generic
browser singleton socket. Excluded: Claude Code's own daemon socket —
seeding a decoy mimicking the tool running the probe is circular, not a
generic dev-machine artifact.

**Linux** (empirical, [ticket 06](06-linux-dev-machine-footprint.md)):
`docker.sock`, an `ssh-agent` socket, one `gpg-agent` socket (one proves
the category, not all 4 variants observed), the dbus system bus socket.
code-server's IPC socket included too, but flagged with a real
implementation dependency: it sits outside every path `ScanSocketRoots`
currently scans, so seeding it is inert until that scanner gains a new
root. Excluded: GUI-only artifacts (ssh-askpass, browser) — never actually
observed on Linux, only reasoned by macOS analogy; not real research to
seed on that basis.

**Windows** ([ticket 07](07-windows-dev-machine-footprint-capture.md)):
`\\.\pipe\openssh-ssh-agent` (fully empirical, real before/after diff),
`\\.\pipe\docker_engine` (included, flagged documented-not-verified —
blocked by the Session 0 wall). VS Code's pipe excluded from v1 — weakest
evidence, pattern-only and version-dependent naming, revisit once someone
actually captures it.

**Processes**: same tool families (Docker/ssh-agent/gpg-agent) map to
representative process-name entries too, reusing
[ticket 02](02-safe-seeding-mechanism-design.md)'s live-process mechanism
rather than needing separate research.

Explicitly a **starting catalogue, not exhaustive** — matching how this
was already framed externally (an open invitation for community
contributions to the seed list), consistent with this map's whole
approach: real research over invented completeness.

**This map's decision-making is now fully done — only
[ticket 10 (write the ADR)](10-write-final-adr.md) remains.**

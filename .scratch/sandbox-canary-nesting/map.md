# Sandbox canary nesting

wayfinder:map

## Destination

A decision (ADR + updated `docs/reporting-site-plan.md`, same artifact shape
as the [seed-ipc-targets map](../seed-ipc-targets/map.md)) for how each
rootfs-swapping `sandbox`-family runtime — `docker`, `podman`, `bwrap`,
`nspawn`, `gvisor` — gets launched as a genuine **child of the seeded
parent host**, using only that runtime's real out-of-the-box default
sharing behavior, with probe-binary-in/report-out plumbing that doesn't
itself alter the isolation boundary.

## The canary model (Chris's framing, load-bearing for every ticket)

Seed canaries in the **parent** (the real, unconfined seeded host — already
what `seed-decoys.sh` does). Launch the sandbox as a **child of** that
seeded parent, not a disconnected, freshly-built environment with no
relationship to it. The test: can the process *inside* the sandbox reach
the canary that exists *outside*, in the parent it's nested in? A sandbox
that was never actually nested in the seeded parent can't produce a
meaningful answer either way.

**Do not seed canaries inside the sandbox.** That tests something else
(whether the sandbox's own isolated environment happens to contain
artifacts) and misrepresents the real threat model — a real attacker inside
a real sandbox is trying to reach *out*, not finding things planted *in*.
The sandbox gets tested exactly as the vendor ships it: nothing artificially
opened up, nothing artificially closed down beyond its own default
configuration.

## Why this map exists (how it was found)

Traced live during the `docker-namespace-tightness-fix` session: the
`sandbox`-family rows in `scan-matrix.yaml` (`scripts/run-probe-in-sandbox.sh`)
construct a **fresh, disconnected environment** per runtime rather than
nesting inside the seeded parent:
- `docker`/`podman`: `docker run --network=none -v "$PWD:/work" ... ubuntu:latest` —
  a brand-new image, no `$HOME`, `--network=none` and the `$PWD`-only mount
  are the test harness's own additions (for probe-binary-in/report-out),
  not Docker's/Podman's defaults.
- `bwrap`: constructs a new root from `/usr`, `/bin`, `/etc` + `$PWD` only —
  no `/home` at all.
- `nspawn`/`gvisor`: use a prepared `ROOTFS` built fresh from a
  `docker export` of an `ubuntu:latest` image — no relation to the seeded
  parent whatsoever.

None of these five are children of the seeded parent; they're siblings that
never had a route to it. So the existing baseline-normalized comparison
(today: files via `sensitive_readable_paths`/`writeable_paths`; once
[seed-ipc-targets](../seed-ipc-targets/map.md) lands: sockets/pipes/processes
too) has likely **never been meaningful for these five rows** — a fresh
environment correctly showing no decoys isn't evidence anything was
blocked; there was never anything there to block.

`srt`, `firejail`, `nono` restrict the *same* filesystem via policy
(seccomp/Landlock/namespaces) rather than swapping in a fresh rootfs, so
they likely already nest correctly — out of scope for the core fix, but
see [the flag-audit ticket](issues/06-firejail-nono-srt-flag-audit.md) for
a smaller, related check.

## Notes

- Domain: same as [seed-ipc-targets](../seed-ipc-targets/map.md) —
  `CONTEXT.md`'s **Baseline vs Sandbox** and **Seed / Decoy** entries. This
  map sharpens "Baseline vs Sandbox" itself: the *methodology* for how a
  sandbox run relates to its baseline, not just what gets seeded into it.
  Update `CONTEXT.md` via `/domain-modeling` if this changes that entry.
- Standing preference: **plan, don't do** — same exception as the sibling
  map, one prototype/verify ticket per runtime family before writing the
  final ADR.
- Relationship to [seed-ipc-targets](../seed-ipc-targets/map.md): that map
  decides *what* gets seeded (sockets, pipes, processes) and *how* to seed
  it safely on a real machine. This map decides *whether the sandboxed run
  can even reach what's seeded on the parent at all*. Its
  [mechanism-design ticket](../seed-ipc-targets/issues/02-safe-seeding-mechanism-design.md)
  should be read alongside whatever this map decides — the nesting fix
  applies to existing file decoys today and will apply to sockets/pipes
  once seeded.
- Test environment: Docker Desktop's `desktop-linux` VM (disposable) covers
  docker/podman/bwrap research directly. nspawn/gvisor need root
  (`sudo`) and are Linux-specific — check what's feasible inside the Docker
  VM (nested privilege) vs. needs a real Linux box.
- Existing code to route through: `scripts/run-probe-in-sandbox.sh` (the
  actual per-runtime invocations), `scripts/seed-decoys.sh` (the existing
  parent-side seeding, unchanged by this map — the fix is entirely about
  how the sandbox gets launched relative to it).

## Decisions so far

- [firejail/nono/srt flag audit](issues/06-firejail-nono-srt-flag-audit.md) — confirms the map's "Out of scope" call: all three stay genuinely nested in the seeded parent filesystem (no fresh-rootfs bug). `srt` matches its vendor secure-by-default exactly. But `firejail --net=none` and `nono --block-net` are project-added, not vendor defaults (both tools are network-open by default) — a smaller, network-only version of the core bug, folded into ticket 08 rather than needing its own fix track.
- [Podman default nesting](issues/02-podman-default-nesting.md) — same structural-absence result as Docker (nothing shared by default), network bridged by default, `podman cp` works with zero mounts. New wrinkle: rootless mode maps container UID 0 to the real invoking host user once something *is* bind-mounted — relevant to how seeded sockets' permissions would resolve once shared in.
- [Docker default nesting](issues/01-docker-default-nesting.md) — confirms both current script flags are non-default: filesystem sharing is genuinely zero by default (good), but `--network=none` is *stricter* than Docker's real default (bridge, working DNS/egress) — the fix isn't just "stop disconnecting the filesystem," it's also "stop over-restricting the network beyond what Docker ships." `docker cp` (create-but-don't-start, then copy) is the non-invasive plumbing mechanism, confirmed both directions.
- [gVisor default nesting](issues/05-gvisor-default-nesting.md) — **fix verified end-to-end, not just designed**: no implicit default (100% explicit via OCI `mounts`), sharing = one more entry on the array the workflow already uses for the output dir, empirically confirmed a decoy goes from invisible → readable byte-for-byte with that one addition, fingerprint still fires correctly either way. Side findings: a stale comment in `run-probe-in-sandbox.sh` (fold into ticket 08), and a genuine separate probe bug — `mounted_volumes_detections` doesn't surface the bind mount even though the file was reachable (flagged to Chris, not blocking this map).
- [nspawn default nesting](issues/04-nspawn-default-nesting.md) — nesting directly on host root is a hard compiled-in refusal (confirmed empirically), but `--bind=`/`--bind-ro=` is the vendor-documented, already-partially-used mechanism to share the seeded parent in. Empirically verified decoy invisible → reachable with one `--bind=` added, fingerprint unaffected. Clear-cut, unlike bwrap.
- [bwrap default nesting](issues/03-bwrap-default-nesting.md) — no formal vendor default exists (bwrap's own README says so), so this genuinely doesn't parallel the container-runtime tickets. `--bind / /` fingerprints correctly and genuinely nests, but is a real security-relevant gap: it's read-write for the *entire* host via ordinary Unix DAC, not namespace-enforced like the current minimal reconstruction's `--ro-bind /etc /etc`. Recommendation: `--ro-bind / /` plus one targeted writable bind for the probe's workdir — least-privilege nesting, not maximum sharing.

**All six per-runtime/tool research tickets now resolved** (01–06).

- [Scope decision: agent harnesses](issues/07-scope-decision-agent-harnesses.md) — **in scope now** (Chris's call): `claude-sandbox`/`codex-sandbox` apply the agent's own bwrap/Seatbelt in-process (likely already fine, unconfirmed); `gemini-docker`/`trae-docker` re-exec the whole CLI inside a *vendor-controlled* docker invocation (not ours to construct), which may turn out to be a legitimate vendor default rather than a bug — different in kind from the original 5. Four new research tickets (11–14) opened; [ticket 08](issues/08-consolidate-nesting-design.md) now also blocked on them.
- [claude-sandbox nesting](issues/11-claude-sandbox-nesting.md) — **already correct, no fix needed.** Empirically confirmed on macOS: sandboxed and unconfined runs see byte-identical `sensitive_readable_paths` (same process tree, same filesystem), while the sandbox is still genuinely active (network dropped to none, fingerprint correct). Same shape as `srt`/`firejail`/`nono`. Bubblewrap/Linux path not yet tested.
- [codex-sandbox nesting](issues/12-codex-sandbox-nesting.md) — **already correct, no fix needed.** Same pattern as claude-sandbox: reads identical on/off, writes genuinely confined to workspace-write's real policy, fingerprint verified against macOS's actual `sandbox_check()` API. Both agent-CLI-native sandboxes now confirmed fine — remaining uncertainty is isolated to the two docker-re-exec cases (13, 14).
- [trae-docker nesting](issues/14-trae-docker-nesting.md) — **already correct, no fix needed, for a different reason than claude/codex.** Read trae-agent's own source directly: it mounts only the working directory, nothing else, with no flag to widen it. A real container on the real Docker daemon on the real seeded host, using trae's own unmodified mount logic — correctly nested by construction, not a harness-built disconnected environment. No extension point exists, and none should be forced (would reintroduce the exact anti-pattern this map exists to fix).
- [gemini-docker nesting](issues/13-gemini-docker-nesting.md) — **already correct, no fix needed.** Read gemini-cli's own source: workspace bind-mounted at its real host path plus a small named allowlist (`.gemini`, tmp, conditionally gcloud), full `$HOME` deliberately not shared — a real vendor-default boundary the probe's decoys correctly fall outside of. `SANDBOX_FLAGS` correctly stays mock-plumbing-only.

**All 10 tickets ticket 08 was blocked on (01–06, 11–14) now resolved.** Net result across the four agent-harness tickets: zero script changes needed — three were already correctly nested for three different reasons, one (gemini) tests a real, narrower vendor boundary that shouldn't be widened. [Ticket 08](issues/08-consolidate-nesting-design.md) is fully unblocked and now scoped entirely to the original 5 generic runtimes (docker/podman/bwrap/nspawn/gvisor) plus the small firejail/nono network-flag fix.

## Not yet specified

(none — the agent-harness question graduated into tickets 07/11–14, all resolved)

## Out of scope

- `srt`, `firejail`, `nono` — restrict the same filesystem rather than
  swapping in a fresh rootfs, so the core "never nested" bug doesn't apply.
  [Ticket 06](issues/06-firejail-nono-srt-flag-audit.md) covers a smaller,
  related question (are their restriction flags genuinely default) without
  reopening the core question for these three.
- **`docker`/`podman`/`bwrap`/`nspawn`/`gvisor` mount-flag fix (ruled out
  during [ticket 08](issues/08-consolidate-nesting-design.md))**: any
  sharing flags this project's own script adds to these are our own
  choice, not a vendor's — testing "did the sandbox block what we chose to
  expose" is circular no matter how it's resolved. These don't belong in
  the comparison matrix without something making a real, external
  configuration decision. See the new
  [profile-attestation map](../profile-attestation/map.md), spawned
  directly from this finding.

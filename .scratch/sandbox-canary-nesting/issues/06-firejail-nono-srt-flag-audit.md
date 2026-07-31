Type: research
Status: resolved

## Question

`srt`, `firejail`, and `nono` are ruled out of the core nesting bug (they
restrict the same filesystem rather than swapping in a fresh rootfs — see
the map's "Out of scope"), but the current invocations each add explicit
restriction flags worth a smaller, separate sanity check:
- `firejail --quiet --net=none --seccomp` — is `--net=none` firejail's own
  documented default, or an addition beyond it (mirroring the exact mistake
  found in the docker/podman scripts)?
- `nono run --silent --allow-cwd --allow "$(dirname "$OUT_ABS")" --block-net`
  — same question for `--block-net` and the `--allow` scoping.
- `srt --settings "$SETTINGS"` with a deny-by-default policy allowing only
  `${PWD}`/`/tmp`/`/private/tmp` — same question.

For each: does the added restriction reflect that tool's own recommended/
typical default configuration (in which case it's fine, these three
correctly nest in the seeded parent AND apply a real default policy), or
is it a project-added narrowing that makes the test stricter than the
vendor's actual out-of-the-box behavior (in which case it has the same
"testing an artificial configuration" problem as docker/podman/bwrap, just
without the fresh-rootfs symptom)?

This is a smaller, bounded check — not expected to surface new fog, just
confirm or flag each of the three.

## Answer

Full write-up: `docs/research/firejail-nono-srt-flag-audit.md` on branch
`research/firejail-nono-srt-audit` (commit `e5af3a4`).

Verdicts, checked against each tool's own docs and empirically (disposable
`ubuntu:22.04` container on Docker Desktop's VM):

- **`srt`** — matches its vendor default exactly. `srt`'s README states
  secure-by-default (network deny-all, filesystem write deny-all, read
  allow-all) — confirmed by running bare `srt` with no settings file
  present. The script's JSON policy only widens writes to `$PWD`/`/tmp`,
  the minimum needed to emit a report. No added narrowing.
- **`firejail`** — `--net=none` and `--seccomp` are **project-added**, not
  defaults. Bare `firejail -- curl ...` has open network and full
  filesystem read/write. Confirmed against firejail's own man page. Same
  category of issue as docker/podman, but scoped to network/seccomp only —
  firejail still shares the real host filesystem, so it stays genuinely
  nested.
- **`nono`** — `--block-net` is project-added (nono's own `--help` says
  "Block outbound network access (**allowed by default**)"). Filesystem is
  the inverse: nono denies *everything* with zero flags (won't even start
  non-interactively), so `--allow-cwd`/`--allow <dir>` are the minimum
  grant to run at all, not a narrowing.

**Net effect**: all three remain correctly nested in the seeded parent
filesystem (no fresh-rootfs bug — [ticket 06](#) confirms the map's "Out of
scope" classification holds for the filesystem dimension). But firejail's
and nono's *network*-restriction flags test a policy the vendor doesn't
apply by default — a narrower, single-dimension version of the
docker/podman problem. Worth a small fix (drop `--net=none`/`--block-net`
unless there's a reason to keep them) alongside the main 5-runtime fix, but
doesn't need its own research/prototype tickets — small enough to fold into
[ticket 08](08-consolidate-nesting-design.md)'s consolidation.

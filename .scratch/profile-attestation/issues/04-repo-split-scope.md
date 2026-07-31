Type: grilling
Status: resolved

## Question

What exactly moves to a new repository, and what stays in `sandbox-probe`?

Working assumption to confirm/correct: `sandbox-probe` keeps the Go binary
(`cmd/`, `pkg/`, `main.go`) and its own unit/integration tests
(`pkg/tasks/baseline/*_test.go` etc.) — anything needed to build and
validate the probe as a standalone fingerprinting tool. Everything about
*comparing* sandboxes moves out: `scan-matrix.yaml`, `scripts/seed-decoys.sh`,
`scripts/run-probe-in-sandbox.sh` + every agent stub script, `site/`,
`docs/reporting-site-plan.md`, the `docs/adr/` entries about comparison
methodology (0001 and whatever lands from the sibling maps), and the
`tests/*.sh` baseline/sandbox pair scripts.

Open questions this ticket needs to resolve with Chris directly (project
decisions, not research):
1. New repo name and where it lives (`controlplaneio/sandbox-probe-*`?).
2. Does history migrate (git history for the moved files), or does the new
   repo start fresh referencing `sandbox-probe` as a dependency/released
   binary?
3. Versioning/release relationship: does the comparison repo pin a
   `sandbox-probe` release, or build against `main`?
4. Does this split happen before or after the sibling maps
   ([sandbox-canary-nesting](https://github.com/chrisns/sandbox-probe-reports/blob/main/.scratch/sandbox-canary-nesting/map.md),
   [seed-ipc-targets](https://github.com/chrisns/sandbox-probe-reports/blob/main/.scratch/seed-ipc-targets/map.md)) finish landing their
   fixes — i.e. land the fixes here first then migrate, or migrate first
   and land fixes in the new repo?
5. What happens to the two sibling maps' `.scratch/` directories and
   in-flight tickets if a split happens mid-effort?

## Answer

Resolved via grilling, five sub-decisions:

1. **Boundary**: `sandbox-probe` keeps the Go binary (`cmd/`, `pkg/`,
   `main.go`), its own tests, and `cmd/targets.go`/`list-targets` (the
   probe's own registry, not comparison methodology — stays with what it
   describes, same reasoning as everything else here). Everything about
   *comparing* sandboxes moves: `scan-matrix.yaml`, `scripts/seed-decoys.sh`,
   `scripts/run-probe-in-sandbox.sh` + agent stub scripts, `site/`,
   `docs/reporting-site-plan.md`, comparison-methodology ADRs, `tests/*.sh`
   baseline/sandbox pairs.
2. **Name/location**: reuse `controlplaneio/sandbox-probe-reports` (Chris
   has admin) rather than mint a new repo — confirmed dormant (LICENSE,
   README, one `gemini/` sample, last pushed March 2026).
3. **History**: fresh start, no migration. Full history migration (path
   filtering across two unrelated histories) isn't worth it for mostly-
   archaeological value — the wayfinder maps and ADRs are the actual
   record of *why*, and they travel regardless of git history.
4. **Versioning/dependency**: `sandbox-probe-reports` gets its own
   `go.mod` requiring a pinned `sandbox-probe` version — Dependabot's
   native `gomod` ecosystem handles bumps automatically, confirmed to work
   off git tags directly (existing `v1.0.0`/`v1.1.0` tags are sufficient,
   no need to wait on GitHub Release pages). A checkout-pinned `ref:`
   would NOT get Dependabot coverage. This surfaced that releases are
   currently 100% broken — tracked as [ticket 06](06-fix-release-pipeline.md),
   blocking, since a `go.mod` pin needs real releases to pin to.
5. **Sequencing**: split first (once ticket 06 lands), before the sibling
   maps' fixes — landing fixes here first would mean redoing the move
   immediately after. `seed-ipc-targets` and `sandbox-canary-nesting`'s
   `.scratch/` directories move wholesale to the new repo once the split
   executes (100% comparison-side). This map splits at the same moment:
   tickets 04 and 06 (sandbox-probe's own boundary/build) stay recorded
   here; everything from ticket 05 onward becomes its own map in
   `sandbox-probe-reports`.

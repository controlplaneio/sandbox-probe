# Reporting site — implementation checklist

Sequenced so the data track lands before the page needs anything to render.
Language and rationale: see [CONTEXT.md](../CONTEXT.md) and
[ADR 0001](adr/0001-client-side-site-over-data-branch.md).

## Track 1 — probe: expose the target registry

- [ ] Add `sandbox-probe list-targets` — emit the probe's own checked targets as
      JSON (start with filesystem paths from the sensitive/writable path tasks).
      Probe is the single source of truth; the seeder reads this so it can't drift.
- [ ] Test asserting `list-targets` covers every path the path tasks actually check
      (parity guard).

## Track 2 — seeder (prerequisite: site is hollow without it)

- [ ] A seed step that reads `list-targets` and **soft-plants** a decoy at each
      path — write only where nothing exists (never clobber a real secret).
- [ ] Wire it into the stub plumbing (`scripts/stub-common.sh`) so it runs
      **identically before the baseline and every sandbox run**. Parity is
      load-bearing: seeding one side only produces false 🟩 wins.
- [ ] (Fast-follow, deferred) network/socket decoys: a decoy listening port, a
      fake `docker.sock`, a stub egress target — turns those categories from ⬜ to
      provable. Needs per-runtime plumbing; not blocking.

## Track 3 — publish pipeline (in scan-matrix.yaml)

- [ ] Add a final `aggregate` job, `needs: [build, scan]`, `if: always()`:
  - [ ] download all report artifacts
  - [ ] one commit → `data` branch at `data/<run-timestamp>/<os>-<harness>.json`
  - [ ] rebuild `all-reports.json` by concatenating every report on the branch
  - [ ] `actions/upload-pages-artifact` (site/ + all-reports.json) →
        `actions/deploy-pages`
- [ ] No new triggers — rides the existing weekly cron + dispatch + `matrix/**`.
- [ ] One-time: create the orphan `data` branch; enable Pages (GitHub Actions
      source).

## Track 4 — the page (site/, vanilla JS, no build)

All derivation client-side from `all-reports.json`:

- [ ] Parse reports → group by identity `(os, harness)` → collapse to fingerprint
      points (latest wins).
- [ ] Baseline-normalize each cell against the `direct` report of the same
      `(run-timestamp, os)`; flag unprovable when no baseline.
- [ ] Categorize findings into the 8 capability categories (+ Other for unmapped);
      Privileged uses absolute euid-0 rule.
- [ ] **Matrix view** — identities × categories, 3-state cells, enforcement +
      root badges, ▲/▼ change markers.
- [ ] **Drill-down** — cell → actual values (paths/hosts/ports).
- [ ] **Flip-log** — chronological flips, each attributed to the moved fingerprint
      component.
- [ ] **Charts (ECharts via CDN + SRI, progressive enhancement)** — exposure
      step-line (0–8, calendar x, version-release markLines, multi-identity
      overlay) + per-capability status heatmap.
- [ ] Develop against the existing `reports/` fixtures as a local sample until the
      first real aggregate run exists.

## Extendability check (must stay true)

- New harness → new row/line, automatic (identity from data).
- New finding type → mapped category, or **Other** (never dropped).
- New target → add to probe registry; seeder picks it up via `list-targets`.

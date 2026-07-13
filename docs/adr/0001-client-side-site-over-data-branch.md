# 1. Client-side reporting site over an orphan data branch, baseline-normalized

Date: 2026-07-13

## Status

Accepted

## Context

We want a webpage that lets someone (a) compare AI-coding-agent sandboxes to make
an informed trust decision, (b) track degradations/improvements as harness and
supporting-tech versions move, and (c) absorb new harnesses without code changes.
It must be static, published to GitHub Pages, with all interaction client-side.

Three forces shaped the decision:

- **History has to persist somewhere.** Tracking change over time needs more than
  one snapshot per harness, but each Pages deploy fully replaces the previous one,
  so nothing accumulates on the Pages side by itself.
- **GitHub Actions artifacts are not a usable live data source for an anonymous
  static page.** The download endpoint requires an authenticated `actions:read`
  token even on public repos, returns a ZIP via a 302 to signed storage, and
  artifacts expire (≤90 days). A credential-free client cannot fetch them, and the
  history would silently truncate.
- **Findings are only meaningful relative to a baseline.** A finding's *absence*
  means "blocked" only if the capability was achievable on that host at all. On a
  bare runner most sensitive paths do not exist, so raw presence/absence would make
  every sandbox look strong for the wrong reason.

## Decision

- **Storage:** each `scan-matrix` run commits its per-run report JSON onto an
  orphan `gh-pages`/`data` branch (not artifacts, not `main`). History lives in
  git; `main`'s log stays free of weekly bot commits. A build step bundles all
  snapshots into one manifest the page fetches **same-origin** — no CORS, no auth,
  no expiry.
- **Client-side only:** a single static page (vanilla JS, no framework/build)
  derives the harness list, OS list, finding types, and all views from the
  manifest, so a new harness appears with zero page changes.
- **Time-series model:** identity is the tuple `(os, harness)`. A plotted point is
  a distinct configuration **fingerprint** (`harness version + probe commit +
  kernel release + OS release`); runs sharing a fingerprint collapse to one point,
  latest wins on collision. The axis is a sequence of distinct configurations, not
  wall-clock time.
- **Baseline normalization:** every capability cell is three-state against the
  same-OS unconfined `direct` run — 🟥 leaked, 🟩 blocked, ⬜ n/a.
- **Non-hollow data:** the probe exports its own target registry
  (`list-targets`); a seeder soft-plants decoys (write only where absent) at those
  paths, identically in the baseline and every sandbox run, so blocking is
  provable rather than n/a.

## Consequences

- The site is deployable to plain GitHub Pages with no backend and no secrets.
- Accumulated history is bounded only by git, not artifact retention.
- Changing the fingerprint key, storage layout, or normalization semantics later
  means migrating the accumulated on-branch history and updating both the CI append
  step and the renderer — the reason this is recorded.
- Seed/baseline parity is load-bearing: if a run seeds the baseline but not the
  sandbox (or vice versa), cells flip to false 🟩 wins. CI must guarantee parity.
- Comparability depends on the `direct` baseline existing for each OS; an OS with
  no baseline run cannot be normalized and must be flagged in the UI.

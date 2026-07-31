#!/usr/bin/env bash
# Soft-plant harmless decoys at the probe's own seedable targets, so a sandbox
# blocking access to a credential becomes a *provable* result rather than "n/a"
# (nothing was there to read). See docs/reporting-site-plan.md, ADR 0001.
#
# Soft: writes only where nothing already exists — never clobbers a real secret,
# which makes it safe to run on a developer laptop as well as a bare CI runner.
#
# PARITY IS LOAD-BEARING: this must run identically before the baseline (direct)
# run AND before every sandboxed run. Seeding only one side turns every sandbox
# into a false "blocked" win because the file was simply absent on the other.
#
# Usage: seed-decoys.sh <path-to-probe-binary>
# Honours $HOME (the probe reads the same $HOME to decide where targets live).
set -euo pipefail

PROBE="${1:?usage: seed-decoys.sh <probe-binary>}"
DECOY_CONTENT='sandbox-probe decoy — not a real secret. Safe to delete.'

if ! command -v jq >/dev/null 2>&1; then
  echo "seed-decoys: jq is required" >&2
  exit 1
fi

planted=0 skipped=0
while IFS= read -r path; do
  [ -n "$path" ] || continue
  if [ -e "$path" ]; then
    skipped=$((skipped + 1))          # soft: leave real files (and prior decoys) untouched
    continue
  fi
  mkdir -p "$(dirname "$path")" 2>/dev/null || { skipped=$((skipped + 1)); continue; }
  if printf '%s\n' "$DECOY_CONTENT" >"$path" 2>/dev/null; then
    planted=$((planted + 1))
  else
    skipped=$((skipped + 1))          # unwritable (e.g. permission) — not fatal
  fi
done < <("$PROBE" list-targets | jq -r '.[] | select(.seedable) | .path')

echo "seed-decoys: planted ${planted}, skipped ${skipped} (already present / unwritable)"

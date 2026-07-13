#!/usr/bin/env bash
# Aggregate one scan-matrix run: append the run's reports to the data branch
# (immutable history), rebuild the concatenated all-reports.json from the whole
# branch, and assemble the Pages payload (site/ + data). See ADR 0001.
#
# Runs in the aggregate job after a checkout of main (persist-credentials on, so
# the origin remote is token-authenticated). Expects the run's reports already
# downloaded under incoming/ (from `report-*` artifacts, merge-multiple).
set -euo pipefail

DATA_BRANCH="${DATA_BRANCH:-gh-pages-data}"
RUN_TS="${RUN_TS:-$(date -u +%Y-%m-%dT%H-%M-%SZ)}"   # filesystem-safe; ISO reconstructed at build
ROOT="$(pwd)"
WT="$(mktemp -d)"

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

# Attach the data branch as a worktree (reuses the authenticated origin), or
# start it as an orphan on first ever run.
if git ls-remote --exit-code --heads origin "$DATA_BRANCH" >/dev/null 2>&1; then
  git fetch --depth 200 origin "$DATA_BRANCH"
  git worktree add "$WT" "origin/$DATA_BRANCH"
  git -C "$WT" checkout -B "$DATA_BRANCH"
else
  git worktree add --orphan -b "$DATA_BRANCH" "$WT"
fi

# Append this run's reports. The report-* artifacts flatten to incoming/reports/<os>-<harness>.json.
dest="$WT/data/$RUN_TS"
mkdir -p "$dest"
shopt -s nullglob
reports=(incoming/reports/*.json incoming/*.json)
if [ "${#reports[@]}" -eq 0 ]; then
  echo "publish-site: no incoming reports found" >&2
else
  cp "${reports[@]}" "$dest"/
fi

if git -C "$WT" add data && ! git -C "$WT" diff --cached --quiet; then
  git -C "$WT" commit -m "data: scan run $RUN_TS"
  git -C "$WT" push origin "$DATA_BRANCH"
else
  echo "publish-site: nothing new to commit"
fi

# Rebuild concatenated site data from the full history and assemble Pages payload.
node "$ROOT/scripts/build-site-data.mjs" "$WT/data" > "$ROOT/all-reports.json"
mkdir -p "$ROOT/public"
cp -r "$ROOT"/site/* "$ROOT/public/"
cp "$ROOT/all-reports.json" "$ROOT/public/all-reports.json"
echo "publish-site: published $(node -e 'console.log(require("./all-reports.json").length)') reports"

#!/usr/bin/env bash

# Run the real Gemini CLI through the deterministic local model stub. This
# preserves the legacy report contract without requiring interactive input or a
# user's Gemini configuration.
set -euo pipefail

# shellcheck disable=SC1091 # dynamic script-relative helper path.
source "$(dirname "$0")/runner-common.sh"
validate_runner_inputs "$@" || exit $?
PROBE=$1
OUTDIR=$2
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPORT="reports/sandbox-gemini-podman-$$.json"

trap 'rm -f "$PROJECT_ROOT/$REPORT"' EXIT

mkdir -p "$OUTDIR"
cd "$PROJECT_ROOT"

PROBE="$PROJECT_ROOT/$PROBE" OUT="$REPORT" RUNNER="$(uname -s)" \
  GEMINI_SANDBOX=docker SCAN_ARGS="scan --tasksets baseline" \
  bash "$PROJECT_ROOT/scripts/run-probe-via-gemini-stub.sh"

cp "$REPORT" "$OUTDIR/report.json"

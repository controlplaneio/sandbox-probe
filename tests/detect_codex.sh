#!/bin/sh
# Detect that sandbox-probe, run via the REAL codex binary with the model stubbed out
# (no LLM, no API key — see scripts/run-probe-via-codex-stub.sh), is confined by Codex's
# own sandbox: Seatbelt on macOS, bubblewrap on Linux.
#
# Skips gracefully (exit 0) when its dependencies are missing, so it doesn't break
# `make e2etests` on machines without a codex install (or, on Linux, without bubblewrap).
set -e

skip() { echo "SKIP detect_codex: $1"; exit 0; }

command -v codex >/dev/null 2>&1 || skip "codex not installed"
command -v node  >/dev/null 2>&1 || skip "node not installed"
command -v jq    >/dev/null 2>&1 || skip "jq not installed"

# macOS reports "seatbelt"; on Linux the probe can't always fingerprint the bwrap wrapper, so
# accept the kernel enforcement its sandbox adds (no-new-privs / seccomp-filter).
ok='["seatbelt"]'
if [ "$(uname -s)" = "Linux" ]; then
  command -v bwrap >/dev/null 2>&1 || skip "bubblewrap (bwrap) not installed"
  ok='["bubblewrap","no-new-privs","seccomp-filter","landlock"]'
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

[ -x bin/sandbox-probe ] || skip "bin/sandbox-probe not built (run: make build)"

mkdir -p reports
OUT="reports/sandbox-codex.json"
rm -f "$OUT"

PROBE=./bin/sandbox-probe OUT="$OUT" RUNNER="$(uname -s)" CODEX_SANDBOX=on \
SCAN_ARGS="scan --tasks baseline_sandbox_task --tasksets none" \
  bash "${PROJECT_ROOT}/scripts/run-probe-via-codex-stub.sh"

if [ ! -f "$OUT" ]; then
  echo "detect_codex: no report produced ✗"
  exit 1
fi

echo "=== Verifying Codex sandbox engaged (accept: $ok) ==="
pass=$(jq --argjson ok "$ok" 'any(.findings[]; .findingType == "sandbox_detection" and .task == "baseline_sandbox_detector" and (.value as $v | $ok | index($v)))' "$OUT")
if [ "$pass" = "true" ]; then
  echo "Codex sandbox engaged: ✓ Test passed"
  exit 0
else
  echo "Codex sandbox not engaged: ✗ Test failed"
  jq '[.findings[] | select(.findingType == "sandbox_detection") | .value]' "$OUT"
  exit 1
fi

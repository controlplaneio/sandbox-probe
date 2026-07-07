#!/bin/sh
# Detect that sandbox-probe, run via the REAL claude binary with the model stubbed out
# (no LLM, no API key — see scripts/run-probe-via-claude-stub.sh), is confined by Claude
# Code's own sandbox: Seatbelt on macOS, bubblewrap on Linux.
#
# Unlike the other detect_*.sh scripts, this one SKIPS gracefully (exit 0) when its
# dependencies are missing, so it doesn't break `make e2etests` on machines without a
# claude install (or, on Linux, without bubblewrap).
set -e

skip() { echo "SKIP detect_claude: $1"; exit 0; }

command -v claude >/dev/null 2>&1 || skip "claude not installed"
command -v node   >/dev/null 2>&1 || skip "node not installed"
command -v jq     >/dev/null 2>&1 || skip "jq not installed"

# macOS reports "seatbelt"; on Linux the probe can't always fingerprint Claude Code's bwrap
# wrapper, so accept the kernel enforcement its sandbox adds (no-new-privs / seccomp-filter).
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
OUT="reports/sandbox-claude.json"
rm -f "$OUT"

# Run only the sandbox-detector task for a fast check (the full baseline is slow).
PROBE=./bin/sandbox-probe OUT="$OUT" RUNNER="$(uname -s)" \
SCAN_ARGS="scan --tasks baseline_sandbox_task --tasksets none" \
  bash "${PROJECT_ROOT}/scripts/run-probe-via-claude-stub.sh"

if [ ! -f "$OUT" ]; then
  echo "detect_claude: no report produced ✗"
  exit 1
fi

echo "=== Verifying Claude sandbox engaged (accept: $ok) ==="
pass=$(jq --argjson ok "$ok" 'any(.findings[]; .findingType == "sandbox_detection" and .task == "baseline_sandbox_detector" and (.value as $v | $ok | index($v)))' "$OUT")
if [ "$pass" = "true" ]; then
  echo "Claude sandbox engaged: ✓ Test passed"
  exit 0
else
  echo "Claude sandbox not engaged: ✗ Test failed"
  jq '[.findings[] | select(.findingType == "sandbox_detection") | .value]' "$OUT"
  exit 1
fi

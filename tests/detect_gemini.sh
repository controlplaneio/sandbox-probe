#!/bin/sh
# Detect that sandbox-probe, run via the REAL gemini-cli with the model stubbed out
# (no LLM, no API key — see scripts/run-probe-via-gemini-stub.sh), is confined by gemini's
# own sandbox: Seatbelt (sandbox-exec) on macOS, a container (docker) on Linux.
#
# Skips gracefully (exit 0) when its dependencies are missing, so it doesn't break
# `make e2etests` on machines without a gemini install (or, on Linux, without docker).
set -e

skip() { echo "SKIP detect_gemini: $1"; exit 0; }

command -v gemini >/dev/null 2>&1 || skip "gemini not installed"
command -v node   >/dev/null 2>&1 || skip "node not installed"
command -v jq     >/dev/null 2>&1 || skip "jq not installed"

if [ "$(uname -s)" = "Linux" ]; then
  command -v docker >/dev/null 2>&1 || skip "docker not installed"
  SBX=docker; ok='["docker"]'
else
  SBX=sandbox-exec; ok='["seatbelt"]'
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

[ -x bin/sandbox-probe ] || skip "bin/sandbox-probe not built (run: make build)"

mkdir -p reports
OUT="reports/sandbox-gemini.json"
rm -f "$OUT"

PROBE=./bin/sandbox-probe OUT="$OUT" RUNNER="$(uname -s)" GEMINI_SANDBOX="$SBX" \
SCAN_ARGS="scan --tasks baseline_sandbox_task --tasksets none" \
  bash "${PROJECT_ROOT}/scripts/run-probe-via-gemini-stub.sh"

if [ ! -f "$OUT" ]; then
  echo "detect_gemini: no report produced ✗"
  exit 1
fi

echo "=== Verifying Gemini sandbox engaged (accept: $ok) ==="
pass=$(jq --argjson ok "$ok" 'any(.findings[]; .findingType == "sandbox_detection" and .task == "baseline_sandbox_detector" and (.value as $v | $ok | index($v)))' "$OUT")
if [ "$pass" = "true" ]; then
  echo "Gemini sandbox engaged: ✓ Test passed"
  exit 0
else
  echo "Gemini sandbox not engaged: ✗ Test failed"
  jq '[.findings[] | select(.findingType == "sandbox_detection") | .value]' "$OUT"
  exit 1
fi

#!/usr/bin/env bash
# Run sandbox-probe via the REAL cline agent (cline.bot) with the model stubbed out (no LLM, no key).
# The general mock (scripts/mock-agent-api.mjs) answers cline's /v1/chat/completions with a canned
# run_commands tool call that runs the probe. cline has NO OS sandbox — run_commands executes on the
# host — so this is an unconfined "as-is" harness.
#
# cline's run_commands tool kills any command at 30s, so this row runs a fast scan (SCAN_ARGS) rather
# than the full baseline; cline is unconfined (no sandbox to assert), so the point is only that it
# drives the probe keyless.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default the fast sandbox-detector task).
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8795}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasks baseline_sandbox_task --tasksets none}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

# Absolute paths: cline runs run_commands from its own working directory, not the repo root.
PROBE_ABS="$(cd "$(dirname "$PROBE")" && pwd)/$(basename "$PROBE")"
OUT_ABS="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"

VERSION="$(cline --version 2>/dev/null | awk 'NR==1{print}')" || VERSION=unknown
TAGS="runner=${RUNNER},harness=cline,cline=${VERSION},mode=via-cline-stub"

STUB_LOG="$(mktemp)"
PORT="$PORT" \
PROBE_CMD="${PROBE_ABS} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT_ABS}" \
STUB_LOG="$STUB_LOG" \
  node "${PROJECT_ROOT}/scripts/mock-agent-api.mjs" &
STUB_PID=$!
CFG="$(mktemp -d)"
trap 'kill "$STUB_PID" 2>/dev/null || true; rm -rf "$CFG"' EXIT
for _ in $(seq 1 50); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
  sleep 0.1
done

# --data-dir gives cline isolated state (never touches the runner's ~/.cline); the openai-compatible
# provider points at the mock (dummy key). Default act mode + auto-approve runs the tool unattended.
cline auth openai -k dummy -m mock-model -b "http://127.0.0.1:${PORT}/v1" --data-dir "$CFG" >/dev/null 2>&1

echo "::group::cline (stubbed model)"
cline "Run the sandbox probe and then stop." -P openai -m mock-model --data-dir "$CFG" --auto-approve true </dev/null || true
echo "::endgroup::"

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::cline(stub) did not produce ${OUT}"
  exit 1
fi
echo "cline(stub) wrote ${OUT}"

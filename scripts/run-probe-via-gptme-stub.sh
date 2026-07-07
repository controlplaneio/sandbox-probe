#!/usr/bin/env bash
# Run sandbox-probe via the REAL gptme agent (gptme.org) with the model stubbed out (no LLM, no key).
# The general mock (scripts/mock-agent-api.mjs) answers gptme's /v1/chat/completions with a canned
# shell tool call that runs the probe. gptme has NO OS sandbox — the shell tool runs on the host —
# so this is an unconfined "as-is" harness. gptme drives the OpenAI API non-streaming; the mock
# serves both streaming and non-streaming clients.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8793}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

VERSION="$(gptme --version 2>/dev/null | head -1 | awk '{print $2}')"
TAGS="runner=${RUNNER},harness=gptme,gptme=${VERSION},mode=via-gptme-stub"

STUB_LOG="$(mktemp)"
PORT="$PORT" \
PROBE_CMD="${PROBE} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT}" \
STUB_LOG="$STUB_LOG" \
  node "${PROJECT_ROOT}/scripts/mock-agent-api.mjs" &
STUB_PID=$!
CFG="$(mktemp -d)"
trap 'kill "$STUB_PID" 2>/dev/null || true; rm -rf "$CFG"' EXIT
for _ in $(seq 1 50); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
  sleep 0.1
done

# Scratch XDG dirs so the runner's real gptme config/logs are untouched; the openai provider points
# at the mock (dummy key). --tool-format tool = native function-calling (the mock's tool_calls path).
export XDG_CONFIG_HOME="$CFG" XDG_DATA_HOME="$CFG/data" XDG_STATE_HOME="$CFG/state"
export OPENAI_BASE_URL="http://127.0.0.1:${PORT}/v1" OPENAI_API_KEY=dummy GPTME_CHECK_UPDATE=false

echo "::group::gptme (stubbed model)"
gptme -n -y -m openai/mock-model -t shell --tool-format tool --no-stream -w "$PROJECT_ROOT" \
  "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::gptme(stub) did not produce ${OUT}"
  exit 1
fi
echo "gptme(stub) wrote ${OUT}"

#!/usr/bin/env bash
# Run sandbox-probe via the REAL goose agent (block/goose) with the model stubbed out (no LLM, no
# key). The general mock (scripts/mock-agent-api.mjs) answers goose's /v1/chat/completions with a
# canned shell tool call that runs the probe. Goose has NO OS sandbox — it runs the shell tool
# directly on the host — so this is an unconfined "as-is" harness.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8791}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

VERSION="$(goose --version 2>/dev/null | head -1)"
TAGS="runner=${RUNNER},harness=goose,goose=${VERSION},mode=via-goose-stub"

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

# Scratch config; env selects the built-in openai provider pointed at the mock (dummy key).
# GOOSE_MODE=auto auto-approves tool calls; the built-in developer extension provides the shell tool.
export XDG_CONFIG_HOME="$CFG"
export GOOSE_PROVIDER=openai GOOSE_MODEL=mock-model GOOSE_MODE=auto GOOSE_DISABLE_KEYRING=1
export OPENAI_HOST="http://127.0.0.1:${PORT}" OPENAI_API_KEY=dummy

echo "::group::goose (stubbed model)"
goose run --no-session -q -t "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::goose(stub) did not produce ${OUT}"
  exit 1
fi
echo "goose(stub) wrote ${OUT}"

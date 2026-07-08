#!/usr/bin/env bash
# Run sandbox-probe via the REAL pi coding agent (earendil-works/pi) with the model stubbed out (no
# LLM, no key). The general mock (scripts/mock-agent-api.mjs) answers pi's /v1/chat/completions with
# a canned bash tool call that runs the probe. pi has NO OS sandbox — it runs the bash tool directly
# on the host — so this is an unconfined "as-is" harness.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8792}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

VERSION="$(pi --version 2>/dev/null | awk 'NR==1{if(match($0,/[0-9]+\.[0-9][0-9.]*/)) print substr($0,RSTART,RLENGTH)}')" || VERSION=""; VERSION="${VERSION:-unknown}"
TAGS="runner=${RUNNER},harness=pi,pi=${VERSION},mode=via-pi-stub"

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

# Scratch config dir; a custom openai-completions provider points pi at the mock (dummy key). compat
# flags keep pi off the `developer` role / reasoning_effort the mock doesn't implement.
export PI_CODING_AGENT_DIR="$CFG" PI_OFFLINE=1 PI_TELEMETRY=0
cat > "$CFG/models.json" <<JSON
{ "providers": { "mock": {
  "baseUrl": "http://127.0.0.1:${PORT}/v1", "api": "openai-completions", "apiKey": "dummy",
  "compat": { "supportsDeveloperRole": false, "supportsReasoningEffort": false },
  "models": [ { "id": "mock-model" } ] } } }
JSON

echo "::group::pi (stubbed model)"
# -p: non-interactive; --tools bash: only the shell tool the mock drives; --no-session/-nc: no state.
pi -p --provider mock --model mock/mock-model --api-key dummy \
  --tools bash --no-session --no-context-files --offline \
  "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::pi(stub) did not produce ${OUT}"
  exit 1
fi
echo "pi(stub) wrote ${OUT}"

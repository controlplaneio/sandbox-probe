#!/usr/bin/env bash
# Run sandbox-probe via the REAL opencode agent with the model stubbed out (no LLM, no key). The
# general mock (scripts/mock-agent-api.mjs) answers opencode's /v1/chat/completions with a canned
# bash tool call that runs the probe. OpenCode has NO OS sandbox — it runs bash directly on the
# host — so this is an unconfined "as-is" harness (its report shows what an unsandboxed agent sees).
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8790}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

VERSION="$(opencode --version 2>/dev/null | awk 'NR==1{if(match($0,/[0-9]+\.[0-9][0-9.]*/)) print substr($0,RSTART,RLENGTH)}')" || VERSION=""; VERSION="${VERSION:-unknown}"
TAGS="runner=${RUNNER},harness=opencode,opencode=${VERSION},mode=via-opencode-stub"

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

# Scratch config so we never touch the user's ~/.config/opencode. A custom openai-compatible
# provider points opencode at the mock; a dummy key is fine.
export XDG_CONFIG_HOME="$CFG" XDG_DATA_HOME="$CFG/data"
mkdir -p "$CFG/opencode"
cat > "$CFG/opencode/opencode.json" <<JSON
{
  "\$schema": "https://opencode.ai/config.json",
  "permission": { "bash": "allow", "edit": "allow", "webfetch": "allow" },
  "provider": {
    "mock": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Mock",
      "options": { "baseURL": "http://127.0.0.1:${PORT}/v1", "apiKey": "dummy" },
      "models": { "mock-model": { "name": "Mock" } }
    }
  }
}
JSON
export OPENCODE_DISABLE_MODELS_FETCH=true OPENCODE_DISABLE_AUTOUPDATE=true OPENCODE_PURE=true

echo "::group::opencode (stubbed model)"
# --auto auto-approves the bash tool; --title skips a title model call. Nonzero exit is tolerated;
# the report file is the success signal.
opencode run --title probe --auto --model mock/mock-model \
  "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::opencode(stub) did not produce ${OUT}"
  exit 1
fi
echo "opencode(stub) wrote ${OUT}"

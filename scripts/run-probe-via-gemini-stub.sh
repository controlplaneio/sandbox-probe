#!/usr/bin/env bash
# Run sandbox-probe via the REAL gemini-cli with the model stubbed out (no LLM, no key). The
# general mock (scripts/mock-agent-api.mjs) answers gemini's :streamGenerateContent with a canned
# run_shell_command call that runs the probe. GOOGLE_GEMINI_BASE_URL puts gemini-cli into a gateway
# auth mode where the key may be a dummy value.
#
# GEMINI_SANDBOX selects the sandbox: empty = as-is (no sandbox); "sandbox-exec" (macOS Seatbelt);
# "docker"/"podman" (Linux container). The container case re-execs the whole CLI inside the
# container, so the mock is bound to 0.0.0.0 and reached via host.docker.internal.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: GEMINI_SANDBOX (''|sandbox-exec|docker|podman), RUNNER, PORT, MODEL,
#               SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8788}"
GEMINI_SANDBOX="${GEMINI_SANDBOX:-}"
MODEL="${MODEL:-gemini-2.5-flash}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

VERSION="$(gemini --version 2>/dev/null | head -1)"
TAGS="runner=${RUNNER},harness=gemini,sandbox=${GEMINI_SANDBOX:-none},gemini=${VERSION},mode=via-gemini-stub"

# Bind/reach the mock differently for the container sandbox (whole CLI runs inside it).
MOCK_HOST=127.0.0.1; BASE_HOST=localhost; SANDBOX_FLAG=()
case "$GEMINI_SANDBOX" in
  docker|podman)
    MOCK_HOST=0.0.0.0; BASE_HOST=host.docker.internal; SANDBOX_FLAG=(--sandbox)
    export GEMINI_SANDBOX
    export SANDBOX_FLAGS='--add-host host.docker.internal:host-gateway --entrypoint ""'
    ;;
  sandbox-exec)
    SANDBOX_FLAG=(--sandbox); export GEMINI_SANDBOX ;;
esac

STUB_LOG="$(mktemp)"
HOST="$MOCK_HOST" PORT="$PORT" \
PROBE_CMD="${PROBE} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT}" \
STUB_LOG="$STUB_LOG" \
  node "${PROJECT_ROOT}/scripts/mock-agent-api.mjs" &
STUB_PID=$!
trap 'kill "$STUB_PID" 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
  sleep 0.1
done

# API-key auth against the local mock: a dummy key is fine (the mock ignores it) and, with no
# cached OAuth creds, gemini-cli selects API-key auth from GEMINI_API_KEY. The mock injects
# PROBE_CMD, so the prompt text is ignored. yolo auto-approves the shell tool;
# GEMINI_CLI_TRUST_WORKSPACE avoids the folder-trust prompt (its --skip-trust flag was removed).
export GEMINI_API_KEY=dummy
export GOOGLE_GEMINI_BASE_URL="http://${BASE_HOST}:${PORT}"
export GEMINI_CLI_TRUST_WORKSPACE=true

echo "::group::gemini (stubbed model, sandbox=${GEMINI_SANDBOX:-none})"
gemini --approval-mode=yolo --model "$MODEL" \
  --prompt "Run the sandbox probe and then stop." "${SANDBOX_FLAG[@]}" || true
echo "::endgroup::"

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::gemini(stub) did not produce ${OUT}"
  exit 1
fi
echo "gemini(stub) wrote ${OUT}"

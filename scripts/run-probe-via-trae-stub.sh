#!/usr/bin/env bash
# Run sandbox-probe via the REAL trae-agent (bytedance/trae-agent) with the model stubbed out (no LLM,
# no key). trae's OpenAI provider speaks the OpenAI *Responses* API (/v1/responses, non-streaming); the
# general mock (scripts/mock-agent-api.mjs) answers with a canned `bash` tool call that runs the probe,
# then a `task_done` call so trae's step loop ends after one run.
#
# TRAE_DOCKER=off (default) — bash runs on the host: an unconfined "as-is" harness (harness=trae).
# TRAE_DOCKER=on            — trae's --docker-image mode runs bash inside a container, so the probe
#                             fingerprints "docker" (harness=trae-docker). trae mounts --working-dir at
#                             /workspace and bundles its tools with pyinstaller, which needs the source
#                             tree — so it runs with cwd=$TRAE_SRC (the trae-agent checkout).
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS, TRAE_DOCKER (off|on), TRAE_SRC (trae-agent checkout dir),
#               TRAE_CLI (trae-cli path), TRAE_DOCKER_IMAGE (default ubuntu:latest).
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8794}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"
TRAE_DOCKER="${TRAE_DOCKER:-off}"
TRAE_SRC="${TRAE_SRC:-.}"
TRAE_CLI="${TRAE_CLI:-trae-cli}"
TRAE_DOCKER_IMAGE="${TRAE_DOCKER_IMAGE:-ubuntu:latest}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"
PROBE_ABS="$(cd "$(dirname "$PROBE")" && pwd)/$(basename "$PROBE")"
OUT_ABS="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
# trae-cli's own venv bin on PATH so pyinstaller (docker mode) resolves.
case "$TRAE_CLI" in */*) export PATH="$(cd "$(dirname "$TRAE_CLI")" && pwd):$PATH" ;; esac

VERSION="$("$TRAE_CLI" --version 2>/dev/null | awk 'match($0,/[0-9]+\.[0-9][0-9.]*/) && !seen {print substr($0,RSTART,RLENGTH); seen=1}')" || VERSION=""; VERSION="${VERSION:-unknown}"
HARNESS=trae; [ "$TRAE_DOCKER" = "on" ] && HARNESS=trae-docker
TAGS="runner=${RUNNER},harness=${HARNESS},trae=${VERSION},mode=via-trae-stub"

WD="$(mktemp -d)"
CFG="$(mktemp -d)/trae_config.yaml"
STUB_LOG="$(mktemp)"
trap 'kill "${STUB_PID:-}" 2>/dev/null || true; rm -rf "$WD" "$(dirname "$CFG")" "$STUB_LOG"' EXIT

# The bash tool command the mock injects. Host mode uses absolute paths; docker mode uses the
# in-container /workspace mount (trae binds --working-dir there) and the report is copied back after.
if [ "$TRAE_DOCKER" = "on" ]; then
  cp "$PROBE_ABS" "$WD/probe"; chmod +x "$WD/probe"
  PROBE_CMD="/workspace/probe ${SCAN_ARGS} --tags ${TAGS} --output_path /workspace/report.json"
  DOCKER_FLAGS=(--docker-image "$TRAE_DOCKER_IMAGE")
else
  PROBE_CMD="${PROBE_ABS} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT_ABS}"
  DOCKER_FLAGS=()
fi

# Scratch config: an openai provider pointed at the mock (dummy key). trae runs on the HOST, so it
# reaches the mock on localhost even in docker mode (only the bash tool runs in the container).
cat > "$CFG" <<YAML
agents:
    trae_agent:
        enable_lakeview: false
        model: mock
        max_steps: 8
        tools:
            - bash
            - task_done
model_providers:
    openai:
        api_key: dummy
        provider: openai
        base_url: http://127.0.0.1:${PORT}/v1
models:
    mock:
        model_provider: openai
        model: mock-model
        max_tokens: 4096
        temperature: 0
        top_p: 1
        top_k: 0
        max_retries: 1
        parallel_tool_calls: false
YAML

PORT="$PORT" PROBE_CMD="$PROBE_CMD" STUB_LOG="$STUB_LOG" \
  node "${PROJECT_ROOT}/scripts/mock-agent-api.mjs" &
STUB_PID=$!
for _ in $(seq 1 50); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
  sleep 0.1
done

echo "::group::trae (stubbed model, docker=${TRAE_DOCKER})"
# cd into the trae-agent checkout: docker mode's pyinstaller tool-bundling reads trae_agent/tools/*.py
# relative to cwd. Harmless for host mode.
( cd "$TRAE_SRC" && "$TRAE_CLI" run "Run the sandbox probe and then stop." \
    --config-file "$CFG" --working-dir "$WD" "${DOCKER_FLAGS[@]}" </dev/null ) || true
echo "::endgroup::"

# Docker mode: the probe wrote the report into the mounted workspace ($WD/report.json) — copy it out.
[ "$TRAE_DOCKER" = "on" ] && [ -f "$WD/report.json" ] && cp "$WD/report.json" "$OUT_ABS"

echo "== mock request log =="; cat "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::trae(stub, docker=${TRAE_DOCKER}) did not produce ${OUT}"
  exit 1
fi
echo "trae(stub) wrote ${OUT}"

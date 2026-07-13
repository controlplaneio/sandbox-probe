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
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8793}"
stub_init
TAGS="runner=${RUNNER},harness=gptme,gptme=$(stub_semver gptme --version),mode=via-gptme-stub"
stub_start_mock

# Scratch XDG dirs so the runner's real gptme config/logs are untouched; the openai provider points
# at the mock (dummy key). --tool-format tool = native function-calling (the mock's tool_calls path).
CFG="$(mktemp -d)"; STUB_SCRATCH+=("$CFG")
export XDG_CONFIG_HOME="$CFG" XDG_DATA_HOME="$CFG/data" XDG_STATE_HOME="$CFG/state"
export OPENAI_BASE_URL="http://127.0.0.1:${PORT}/v1" OPENAI_API_KEY=dummy GPTME_CHECK_UPDATE=false

echo "::group::gptme (stubbed model)"
gptme -n -y -m openai/mock-model -t shell --tool-format tool --no-stream -w "$PROJECT_ROOT" \
  "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

stub_finish "gptme(stub)"

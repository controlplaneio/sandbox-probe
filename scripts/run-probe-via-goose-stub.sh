#!/usr/bin/env bash
# Run sandbox-probe via the REAL goose agent (block/goose) with the model stubbed out (no LLM, no
# key). The general mock (scripts/mock-agent-api.mjs) answers goose's /v1/chat/completions with a
# canned shell tool call that runs the probe. Goose has NO OS sandbox — it runs the shell tool
# directly on the host — so this is an unconfined "as-is" harness.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8791}"
stub_init
TAGS="runner=${RUNNER},harness=goose,goose=$(stub_semver goose --version),mode=via-goose-stub"
stub_start_mock

# Scratch config; env selects the built-in openai provider pointed at the mock (dummy key).
# GOOSE_MODE=auto auto-approves tool calls; the built-in developer extension provides the shell tool.
CFG="$(mktemp -d)"; STUB_SCRATCH+=("$CFG")
export XDG_CONFIG_HOME="$CFG"
export GOOSE_PROVIDER=openai GOOSE_MODEL=mock-model GOOSE_MODE=auto GOOSE_DISABLE_KEYRING=1
export OPENAI_HOST="http://127.0.0.1:${PORT}" OPENAI_API_KEY=dummy

echo "::group::goose (stubbed model)"
goose run --no-session -q -t "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

stub_finish "goose(stub)"

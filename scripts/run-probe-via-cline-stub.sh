#!/usr/bin/env bash
# Run sandbox-probe via the REAL cline agent (cline.bot) with the model stubbed out (no LLM, no key).
# The general mock (scripts/mock-agent-api.mjs) answers cline's /v1/chat/completions with a canned
# run_commands tool call that runs the probe. cline has NO OS sandbox — run_commands executes on the
# host — so this is an unconfined "as-is" harness.
#
# cline's run_commands tool hard-kills any command at 30s; the full baseline completes in a few
# seconds (the socket scan is bounded to runtime dirs, not the whole disk), so it runs inline.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default the full baseline).
set -eo pipefail
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8795}"
stub_init

# Absolute paths: cline runs run_commands from its own working directory, not the repo root.
PROBE_ABS="$(cd "$(dirname "$PROBE")" && pwd)/$(basename "$PROBE")"
OUT_ABS="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"
TAGS="runner=${RUNNER},harness=cline,cline=$(stub_semver cline --version),mode=via-cline-stub"
PROBE_CMD="${PROBE_ABS} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT_ABS}"
stub_start_mock

# --data-dir gives cline isolated state (never touches the runner's ~/.cline); the openai-compatible
# provider points at the mock (dummy key). Default act mode + auto-approve runs the tool unattended.
CFG="$(mktemp -d)"; STUB_SCRATCH+=("$CFG")
cline auth openai -k dummy -m mock-model -b "http://127.0.0.1:${PORT}/v1" --data-dir "$CFG" >/dev/null 2>&1

echo "::group::cline (stubbed model)"
cline "Run the sandbox probe and then stop." -P openai -m mock-model --data-dir "$CFG" --auto-approve true </dev/null || true
echo "::endgroup::"

stub_finish "cline(stub)"

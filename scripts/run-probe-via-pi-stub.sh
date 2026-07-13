#!/usr/bin/env bash
# Run sandbox-probe via the REAL pi coding agent (earendil-works/pi) with the model stubbed out (no
# LLM, no key). The general mock (scripts/mock-agent-api.mjs) answers pi's /v1/chat/completions with
# a canned bash tool call that runs the probe. pi has NO OS sandbox — it runs the bash tool directly
# on the host — so this is an unconfined "as-is" harness.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8792}"
stub_init
TAGS="runner=${RUNNER},harness=pi,pi=$(stub_semver pi --version),mode=via-pi-stub"
stub_start_mock

# Scratch config dir; a custom openai-completions provider points pi at the mock (dummy key). compat
# flags keep pi off the `developer` role / reasoning_effort the mock doesn't implement.
CFG="$(mktemp -d)"; STUB_SCRATCH+=("$CFG")
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

stub_finish "pi(stub)"

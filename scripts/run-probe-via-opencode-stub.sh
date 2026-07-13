#!/usr/bin/env bash
# Run sandbox-probe via the REAL opencode agent with the model stubbed out (no LLM, no key). The
# general mock (scripts/mock-agent-api.mjs) answers opencode's /v1/chat/completions with a canned
# bash tool call that runs the probe. OpenCode has NO OS sandbox — it runs bash directly on the
# host — so this is an unconfined "as-is" harness (its report shows what an unsandboxed agent sees).
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: RUNNER, PORT, SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8790}"
stub_init
TAGS="runner=${RUNNER},harness=opencode,opencode=$(stub_semver opencode --version),mode=via-opencode-stub"
stub_start_mock

# Scratch config so we never touch the user's ~/.config/opencode. A custom openai-compatible
# provider points opencode at the mock; a dummy key is fine.
CFG="$(mktemp -d)"; STUB_SCRATCH+=("$CFG")
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

stub_finish "opencode(stub)"

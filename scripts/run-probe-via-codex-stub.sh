#!/usr/bin/env bash
# Run sandbox-probe via the REAL codex CLI with the model stubbed out (no LLM, no key). The general
# mock (scripts/mock-agent-api.mjs) answers codex's /v1/responses with a canned shell tool call that
# runs the probe; codex runs it inside its own sandbox — Seatbelt on macOS, bubblewrap+seccomp on
# Linux. Codex sandboxes only the shell child, so the mock stays reachable on localhost.
#
# CODEX_SANDBOX=on (default) confines the run with `workspace-write` (writes limited to the work
# dir, no network — the probe can still emit its report). CODEX_SANDBOX=off runs it "as is"
# (approvals + sandbox bypassed), an unconfined baseline to diff against.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: CODEX_SANDBOX (on|off, default on), RUNNER, PORT,
#               SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8789}"
CODEX_SANDBOX="${CODEX_SANDBOX:-on}"
stub_init

VERSION="$(stub_semver codex --version)"
# When confining on Linux, Codex wraps the probe in bubblewrap — record bwrap's version too (the
# sandbox engine the report reflects; kernel/OS is in the environment_detection finding). No-op on
# macOS (Seatbelt) / when unconfined.
SANDBOX_TOOL_TAG=""
if [ "$CODEX_SANDBOX" = "on" ] && command -v bwrap >/dev/null 2>&1; then
  BWRAP_VERSION="$(bwrap --version 2>/dev/null | awk 'NR==1{print $NF}')" || BWRAP_VERSION=""
  [ -n "$BWRAP_VERSION" ] && SANDBOX_TOOL_TAG=",bwrap=${BWRAP_VERSION}"
fi
TAGS="runner=${RUNNER},harness=codex,sandbox=${CODEX_SANDBOX},codex=${VERSION}${SANDBOX_TOOL_TAG},mode=via-codex-stub"

export BASH_TIMEOUT_MS=600000   # carried by the mock into the tool input
stub_start_mock

# Scratch config so we never touch the runner's real ~/.codex; dummy key for the mock provider.
CODEX_HOME="$(mktemp -d)"; export CODEX_HOME; STUB_SCRATCH+=("$CODEX_HOME")
export MOCK_KEY=dummy OPENAI_API_KEY=dummy

# Confined = workspace-write (fs write limited to cwd, no network; the probe can still write its
# report). As-is = bypass approvals + sandbox entirely.
if [ "$CODEX_SANDBOX" = "on" ]; then SBX=(--sandbox workspace-write); else SBX=(--dangerously-bypass-approvals-and-sandbox); fi
PROVIDER="model_providers.mock={ name = \"mock\", base_url = \"http://127.0.0.1:${PORT}/v1\", wire_api = \"responses\", env_key = \"MOCK_KEY\", request_max_retries = 0, stream_max_retries = 0 }"

echo "::group::codex (stubbed model, sandbox=${CODEX_SANDBOX})"
codex exec --skip-git-repo-check "${SBX[@]}" \
  -c approval_policy="never" \
  -c model_provider="mock" \
  -c "$PROVIDER" \
  -c model="mock-model" \
  "Run the sandbox probe and then stop." </dev/null || true
echo "::endgroup::"

stub_finish "codex(stub)"

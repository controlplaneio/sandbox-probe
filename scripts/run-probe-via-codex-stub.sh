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

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8789}"
CODEX_SANDBOX="${CODEX_SANDBOX:-on}"
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
mkdir -p "$(dirname "$OUT")"

# Extract a whitespace-free version token (codex prints "codex-cli 0.143.0"; a space would word-split
# the shell command the mock runs and truncate the tag list). match() reads to EOF so it's SIGPIPE-safe.
VERSION="$(codex --version 2>/dev/null | awk 'NR==1{if(match($0,/[0-9]+\.[0-9][0-9.]*/)) print substr($0,RSTART,RLENGTH)}')" || VERSION=""; VERSION="${VERSION:-unknown}"
# When confining on Linux, Codex wraps the probe in bubblewrap — record bwrap's version too (the
# sandbox engine the report reflects; kernel/OS is in the environment_detection finding). No-op on
# macOS (Seatbelt) / when unconfined.
SANDBOX_TOOL_TAG=""
if [ "$CODEX_SANDBOX" = "on" ] && command -v bwrap >/dev/null 2>&1; then
  BWRAP_VERSION="$(bwrap --version 2>/dev/null | awk 'NR==1{print $NF}')" || BWRAP_VERSION=""
  [ -n "$BWRAP_VERSION" ] && SANDBOX_TOOL_TAG=",bwrap=${BWRAP_VERSION}"
fi
TAGS="runner=${RUNNER},harness=codex,sandbox=${CODEX_SANDBOX},codex=${VERSION}${SANDBOX_TOOL_TAG},mode=via-codex-stub"

STUB_LOG="$(mktemp)"
PORT="$PORT" \
PROBE_CMD="${PROBE} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT}" \
BASH_TIMEOUT_MS=600000 \
STUB_LOG="$STUB_LOG" \
  node "${PROJECT_ROOT}/scripts/mock-agent-api.mjs" &
STUB_PID=$!
trap 'kill "$STUB_PID" 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
  sleep 0.1
done

# Scratch config so we never touch the runner's real ~/.codex; dummy key for the mock provider.
CODEX_HOME="$(mktemp -d)"; export CODEX_HOME
export MOCK_KEY=dummy
export OPENAI_API_KEY=dummy

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

echo "== mock request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true
rm -rf "$CODEX_HOME" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::codex(stub) did not produce ${OUT}"
  exit 1
fi
echo "codex(stub) wrote ${OUT}"

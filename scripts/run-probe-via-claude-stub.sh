#!/usr/bin/env bash
# Run sandbox-probe inside REAL Claude Code's sandbox, with the model stubbed out (no LLM,
# no API key, no tokens). A tiny local server (scripts/anthropic-stub.mjs) speaks the
# Anthropic Messages API and returns a canned Bash tool_use that runs the probe; Claude Code
# executes that command inside its own sandbox (bubblewrap on Linux, Seatbelt on macOS).
#
# Only the probe (the Bash child) is sandboxed; the claude<->stub traffic is plain localhost
# outside the sandbox, so the probe still measures Claude Code's real egress denial.
#
# Required env: PROBE (path to the probe binary), OUT (report output path).
# Optional env: RUNNER (label for tags), PORT (stub port, default 8787).
set -eo pipefail

: "${PROBE:?PROBE (probe binary path) is required}"
: "${OUT:?OUT (report output path) is required}"
RUNNER="${RUNNER:-$(uname -s)}"
PORT="${PORT:-8787}"
# The scan sub-command + taskset selection the probe runs inside the sandbox. Override to run
# a quick subset locally, e.g. SCAN_ARGS="scan --tasks baseline_sandbox_task --tasksets none".
SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

mkdir -p "$(dirname "$OUT")"

VERSION="$(claude --version 2>/dev/null | awk '{print $1}')"
TAGS="runner=${RUNNER},agent=claude,claude=${VERSION},mode=via-claude-stub"

# A full baseline scan can take a few minutes, so lift Claude Code's Bash timeout well above
# the ~2 min default (env caps + an explicit per-command timeout carried in the tool input).
BASH_TIMEOUT_MS="${BASH_TIMEOUT_MS:-600000}"
export BASH_DEFAULT_TIMEOUT_MS="$BASH_TIMEOUT_MS"
export BASH_MAX_TIMEOUT_MS="$BASH_TIMEOUT_MS"

# Start the Anthropic API stub. It never contacts a real model.
STUB_LOG="$(mktemp)"
PORT="$PORT" \
PROBE_CMD="${PROBE} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT}" \
BASH_TIMEOUT_MS="$BASH_TIMEOUT_MS" \
STUB_LOG="$STUB_LOG" \
  node "${PROJECT_ROOT}/scripts/anthropic-stub.mjs" &
STUB_PID=$!
trap 'kill "$STUB_PID" 2>/dev/null || true' EXIT

# Wait (up to ~5s) for the stub to accept connections, using bash's /dev/tcp.
for _ in $(seq 1 50); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
  sleep 0.1
done

export ANTHROPIC_BASE_URL="http://127.0.0.1:${PORT}"
export ANTHROPIC_API_KEY="sk-ant-stub"              # any non-empty value; the stub ignores it
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1   # no autoupdate/telemetry/error-report egress

echo "::group::claude (stubbed model)"
# bypassPermissions makes the OS sandbox the ONLY constraint on the probe, and sidesteps auto
# mode's classifier (which makes its own model call the stub can't satisfy). The sandbox is
# enabled and made mandatory by --settings, independent of permission mode. It is refused when
# running as root, but GitHub-hosted runners are non-root. Nonzero exit is fine — the report is
# the success signal.
claude \
  --settings "${PROJECT_ROOT}/scripts/config/claude-code-sandbox.json" \
  --permission-mode bypassPermissions \
  --allowedTools "Bash" \
  -p "Run the sandbox probe and then stop." || true
echo "::endgroup::"

echo "== anthropic-stub request log =="
cat "$STUB_LOG" 2>/dev/null || true
rm -f "$STUB_LOG" 2>/dev/null || true

if [ ! -f "$OUT" ]; then
  echo "::error::claude(stub) did not produce ${OUT}"
  exit 1
fi
echo "claude(stub) wrote ${OUT}"

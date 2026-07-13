# shellcheck shell=bash
# Shared plumbing for scripts/run-probe-via-*-stub.sh — the mechanical boilerplate every agent stub
# repeats. A stub sources this, then supplies only what's agent-specific: the version command, the
# harness TAGS, the provider config, and the CLI invocation. Not executable; source it:
#     source "$(dirname "$0")/lib/stub-common.sh"
#
# Contract: set PROBE, OUT (required) and optionally PORT, RUNNER, SCAN_ARGS, MOCK_HOST, PROBE_CMD.
#   stub_init                 validate inputs; set RUNNER/SCAN_ARGS; mkdir OUT's dir
#   stub_semver <cmd...>      print a whitespace-free version token (or "unknown")
#   stub_start_mock           launch the stubbed model API, arm cleanup, block until it's listening
#   stub_finish <label>       dump the mock request log; assert the probe wrote OUT
# Register scratch to remove with STUB_SCRATCH+=(dir); for anything fancier (e.g. restoring a moved
# file) define a stub_extra_cleanup function — the EXIT trap runs it before removing STUB_SCRATCH.

# The mock API + repo root, from this lib's own location (scripts/lib/…) — independent of the caller.
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MOCK="${PROJECT_ROOT}/scripts/mock-agent-api.mjs"

STUB_SCRATCH=()
_stub_cleanup() {
  kill "${STUB_PID:-}" 2>/dev/null || true
  if declare -F stub_extra_cleanup >/dev/null; then stub_extra_cleanup || true; fi
  [ "${#STUB_SCRATCH[@]}" -gt 0 ] && rm -rf "${STUB_SCRATCH[@]}" 2>/dev/null
  return 0
}

stub_init() {
  : "${PROBE:?PROBE (probe binary path) is required}"
  : "${OUT:?OUT (report output path) is required}"
  RUNNER="${RUNNER:-$(uname -s)}"
  SCAN_ARGS="${SCAN_ARGS:-scan --tasksets baseline}"
  mkdir -p "$(dirname "$OUT")"
}

# stub_semver <version-command...> -> a whitespace-free semver, or "unknown". A tag value with a
# space would word-split the shell command the mock runs and truncate the tags; awk's match() reads
# to EOF, so it's SIGPIPE-safe even when the CLI prints several --version lines.
stub_semver() {
  local v
  v="$("$@" 2>/dev/null | awk 'match($0,/[0-9]+\.[0-9][0-9.]*/) && !seen {print substr($0,RSTART,RLENGTH); seen=1}')" || v=""
  printf '%s' "${v:-unknown}"
}

# stub_start_mock -> launch the stubbed model API and block until it accepts connections. Builds
# PROBE_CMD from PROBE/SCAN_ARGS/TAGS/OUT unless the caller pre-set it; honours MOCK_HOST (default
# 127.0.0.1, 0.0.0.0 to reach it from a container). Export any extra env the mock needs (e.g.
# BASH_TIMEOUT_MS) before calling. Sets STUB_PID and arms the EXIT trap.
stub_start_mock() {
  STUB_LOG="$(mktemp)"; STUB_SCRATCH+=("$STUB_LOG")
  : "${PROBE_CMD:=${PROBE} ${SCAN_ARGS} --tags ${TAGS} --output_path ${OUT}}"
  trap _stub_cleanup EXIT
  HOST="${MOCK_HOST:-127.0.0.1}" PORT="$PORT" PROBE_CMD="$PROBE_CMD" STUB_LOG="$STUB_LOG" \
    node "$MOCK" &
  STUB_PID=$!
  local _
  for _ in $(seq 1 50); do
    if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then exec 3>&- 3<&-; break; fi
    sleep 0.1
  done
}

# stub_finish <label> -> dump the mock request log and assert the probe wrote OUT.
stub_finish() {
  echo "== mock request log =="; cat "${STUB_LOG:-/dev/null}" 2>/dev/null || true
  if [ ! -f "$OUT" ]; then
    echo "::error::$1 did not produce ${OUT}"
    exit 1
  fi
  echo "$1 wrote ${OUT}"
}

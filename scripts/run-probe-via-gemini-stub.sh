#!/usr/bin/env bash
# Run sandbox-probe via the REAL gemini-cli with the model stubbed out (no LLM, no key). The
# general mock (scripts/mock-agent-api.mjs) answers gemini's :streamGenerateContent with a canned
# run_shell_command call that runs the probe. GOOGLE_GEMINI_BASE_URL puts gemini-cli into a gateway
# auth mode where the key may be a dummy value.
#
# GEMINI_SANDBOX selects the sandbox: empty = as-is (no sandbox); "sandbox-exec" (macOS Seatbelt);
# "docker"/"podman" (Linux container). The container case re-execs the whole CLI inside the
# container, so the mock is bound to 0.0.0.0 and reached via host.docker.internal.
#
# Required env: PROBE (probe binary), OUT (report path).
# Optional env: GEMINI_SANDBOX (''|sandbox-exec|docker|podman), RUNNER, PORT, MODEL,
#               SCAN_ARGS (default "scan --tasksets baseline").
set -eo pipefail
source "$(dirname "$0")/stub-common.sh"

PORT="${PORT:-8788}"
GEMINI_SANDBOX="${GEMINI_SANDBOX:-}"
MODEL="${MODEL:-gemini-2.5-flash}"
stub_init

# Force gemini-cli to API-key auth against the mock (0.49+ needs the auth type set explicitly, and
# this overrides any cached OAuth login). A workspace .gemini/settings.json works both directly and
# inside the container sandbox, which mounts the workspace. Back up any existing one; the EXIT trap
# (armed now, before the mock starts) restores it via stub_extra_cleanup.
GEMINI_SETTINGS=".gemini/settings.json"; GEMINI_BACKUP=""
mkdir -p .gemini
[ -f "$GEMINI_SETTINGS" ] && { GEMINI_BACKUP="$(mktemp)"; mv "$GEMINI_SETTINGS" "$GEMINI_BACKUP"; }
printf '{"security":{"auth":{"selectedType":"gemini-api-key"}}}\n' > "$GEMINI_SETTINGS"
stub_extra_cleanup() {
  if [ -n "$GEMINI_BACKUP" ]; then mv "$GEMINI_BACKUP" "$GEMINI_SETTINGS"; else rm -f "$GEMINI_SETTINGS"; rmdir .gemini 2>/dev/null || true; fi
}
trap _stub_cleanup EXIT

VERSION="$(stub_semver gemini --version)"
# For the docker backend, record the docker version too (the sandbox engine the report reflects;
# kernel/OS is in the environment_detection finding). Seatbelt (sandbox-exec) has no separate version.
SANDBOX_TOOL_TAG=""
if [ "${GEMINI_SANDBOX:-}" = "docker" ] && command -v docker >/dev/null 2>&1; then
  DOCKER_VERSION="$(docker --version 2>/dev/null | awk 'NR==1{gsub(/,/,"",$3); print $3}')" || DOCKER_VERSION=""
  [ -n "$DOCKER_VERSION" ] && SANDBOX_TOOL_TAG=",docker=${DOCKER_VERSION}"
fi
TAGS="runner=${RUNNER},harness=gemini,sandbox=${GEMINI_SANDBOX:-none},gemini=${VERSION}${SANDBOX_TOOL_TAG},mode=via-gemini-stub"

# Bind/reach the mock differently for the container sandbox (whole CLI runs inside it).
MOCK_HOST=127.0.0.1; BASE_HOST=localhost; SANDBOX_FLAG=()
case "$GEMINI_SANDBOX" in
  docker|podman)
    MOCK_HOST=0.0.0.0; BASE_HOST=host.docker.internal; SANDBOX_FLAG=(--sandbox)
    export GEMINI_SANDBOX
    # The whole CLI re-execs inside the container: route the mock via host.docker.internal, and pass
    # through the key/base-URL + workspace trust so the mounted .gemini/settings.json auth is honoured.
    export SANDBOX_FLAGS="--add-host host.docker.internal:host-gateway --entrypoint \"\" -e GEMINI_CLI_TRUST_WORKSPACE=true -e GEMINI_API_KEY=dummy -e GOOGLE_GEMINI_BASE_URL=http://host.docker.internal:${PORT}"
    ;;
  sandbox-exec)
    SANDBOX_FLAG=(--sandbox); export GEMINI_SANDBOX ;;
esac

stub_start_mock

# A dummy key is fine (the mock ignores it); the settings file above forces API-key auth. The mock
# injects PROBE_CMD, so the prompt text is ignored. yolo auto-approves the shell tool;
# GEMINI_CLI_TRUST_WORKSPACE avoids the folder-trust prompt (the --skip-trust flag was removed).
export GEMINI_API_KEY=dummy
export GOOGLE_GEMINI_BASE_URL="http://${BASE_HOST}:${PORT}"
export GEMINI_CLI_TRUST_WORKSPACE=true

echo "::group::gemini (stubbed model, sandbox=${GEMINI_SANDBOX:-none})"
gemini --approval-mode=yolo --model "$MODEL" \
  --prompt "Run the sandbox probe and then stop." "${SANDBOX_FLAG[@]}" || true
echo "::endgroup::"

stub_finish "gemini(stub)"

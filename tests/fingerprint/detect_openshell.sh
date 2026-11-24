#!/usr/bin/env bash
# e2e: run sandbox-probe inside a real NVIDIA OpenShell sandbox (docker driver)
# and assert the probe reports sandbox_detection == "openshell".
#
# Skips (exit 0) when the openshell CLI or docker is unavailable, so CI and
# `make e2etests` stay green without the lab installed.
#
# Requires:
#   - `openshell` CLI on PATH, or $OPENSHELL_BIN pointing at it
#   - `openshell-gateway` binary on PATH, or $OPENSHELL_GATEWAY_BIN
#   - docker, with daemon reachable
#
# Known-red against main today: OpenShell has no detector, so the probe's
# generic container-runtime check reports sandbox_detection="docker" instead
# of "openshell". This script documents that gap and will start passing once
# pkg/ gains OpenShell-aware detection.

set -euo pipefail

OPENSHELL_BIN="${OPENSHELL_BIN:-$(command -v openshell || true)}"
OPENSHELL_GATEWAY_BIN="${OPENSHELL_GATEWAY_BIN:-$(command -v openshell-gateway || true)}"

if [ -z "${OPENSHELL_BIN}" ] || [ -z "${OPENSHELL_GATEWAY_BIN}" ]; then
    echo "SKIP: openshell CLI / openshell-gateway binary not found (set OPENSHELL_BIN / OPENSHELL_GATEWAY_BIN, or install from https://github.com/NVIDIA/OpenShell/releases)"
    exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "SKIP: docker not found"
    exit 0
fi

if ! docker info >/dev/null 2>&1; then
    echo "SKIP: docker daemon not reachable"
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

mkdir -p "$HOME/.sandbox-probe/tmp"
WORKDIR=$(mktemp -d -p "$HOME/.sandbox-probe/tmp")
echo "created WORKDIR: ${WORKDIR}"

SANDBOX_IMAGE="${OPENSHELL_E2E_SANDBOX_IMAGE:-ubuntu:24.04}"
SUPERVISOR_IMAGE="${OPENSHELL_E2E_SUPERVISOR_IMAGE:-ghcr.io/nvidia/openshell/supervisor:0.1.4}"
RUNTIME_IMAGE="${OPENSHELL_E2E_RUNTIME_IMAGE:-ghcr.io/nvidia/openshell/sandbox:0.1.4}"
GATEWAY_PORT="${OPENSHELL_E2E_GATEWAY_PORT:-17670}"
HEALTH_PORT="${OPENSHELL_E2E_HEALTH_PORT:-17671}"
SANDBOX_NAME="probe-e2e-$$"

GATEWAY_PID=""

cleanup() {
    local exit_code=$?
    if [ -n "${OPENSHELL_BIN}" ]; then
        XDG_CONFIG_HOME="${WORKDIR}/config" XDG_DATA_HOME="${WORKDIR}/data" XDG_STATE_HOME="${WORKDIR}/state" \
            "${OPENSHELL_BIN}" --gateway-endpoint "http://127.0.0.1:${GATEWAY_PORT}" \
            sandbox delete "${SANDBOX_NAME}" >/dev/null 2>&1 || true
    fi
    if [ -n "${GATEWAY_PID}" ]; then
        kill "${GATEWAY_PID}" >/dev/null 2>&1 || true
        wait "${GATEWAY_PID}" 2>/dev/null || true
    fi
    docker ps -aq --filter "label=openshell.ai/sandbox-namespace=detect-openshell-$$" 2>/dev/null \
        | xargs -r docker rm -f >/dev/null 2>&1 || true
    rm -rf "${WORKDIR}" 2>/dev/null || true
    exit "${exit_code}"
}
trap cleanup EXIT

# --- build a static probe copy -------------------------------------------
PROBE_BIN="${WORKDIR}/sandbox-probe"
if [ -x "${PROJECT_ROOT}/bin/sandbox-probe" ]; then
    cp "${PROJECT_ROOT}/bin/sandbox-probe" "${PROBE_BIN}"
else
    echo "Building static sandbox-probe..."
    (cd "${PROJECT_ROOT}" && CGO_ENABLED=0 go build -o "${PROBE_BIN}" .)
fi

# --- gateway JWT signing material -----------------------------------------
JWT_DIR="${WORKDIR}/jwt"
mkdir -p "${JWT_DIR}"
openssl genpkey -algorithm Ed25519 -out "${JWT_DIR}/signing.pem" >/dev/null 2>&1
openssl pkey -in "${JWT_DIR}/signing.pem" -pubout -out "${JWT_DIR}/public.pem" >/dev/null 2>&1
openssl rand -hex 16 >"${JWT_DIR}/kid"

# --- minimal plaintext gateway + docker compute driver config -------------
GATEWAY_CONFIG="${WORKDIR}/gateway.toml"
cat >"${GATEWAY_CONFIG}" <<EOF
[openshell]
version = 2

[openshell.gateway]
log_level = "info"

[openshell.gateway.auth]
allow_unauthenticated_users = true

[openshell.gateway.gateway_jwt]
signing_key_path = "${JWT_DIR}/signing.pem"
public_key_path = "${JWT_DIR}/public.pem"
kid_path = "${JWT_DIR}/kid"

[openshell.drivers.docker]
allow_driver_config = true
sandbox_label = "detect-openshell-$$"
default_image = "${SANDBOX_IMAGE}"
image_pull_policy = "if_not_present"
enable_bind_mounts = true
sandbox_runtime_image = "${RUNTIME_IMAGE}"
supervisor_image = "${SUPERVISOR_IMAGE}"

[openshell.drivers.docker.resource_admission]
enabled = false
EOF

export XDG_CONFIG_HOME="${WORKDIR}/config"
export XDG_DATA_HOME="${WORKDIR}/data"
export XDG_STATE_HOME="${WORKDIR}/state"
mkdir -p "${XDG_CONFIG_HOME}" "${XDG_DATA_HOME}" "${XDG_STATE_HOME}"

echo "Starting openshell-gateway on 127.0.0.1:${GATEWAY_PORT}..."
"${OPENSHELL_GATEWAY_BIN}" \
    --config "${GATEWAY_CONFIG}" \
    --bind-address 127.0.0.1 \
    --port "${GATEWAY_PORT}" \
    --health-port "${HEALTH_PORT}" \
    --compute-driver docker \
    --disable-tls \
    --db-url "sqlite:${XDG_STATE_HOME}/gateway.db?mode=rwc" \
    >"${WORKDIR}/gateway.log" 2>&1 &
GATEWAY_PID=$!

elapsed=0
until curl -sf "http://127.0.0.1:${HEALTH_PORT}/healthz" >/dev/null 2>&1; do
    if ! kill -0 "${GATEWAY_PID}" 2>/dev/null; then
        echo "ERROR: openshell-gateway exited before becoming healthy" >&2
        cat "${WORKDIR}/gateway.log" >&2
        exit 1
    fi
    elapsed=$((elapsed + 1))
    if [ "${elapsed}" -ge 60 ]; then
        echo "ERROR: gateway did not become healthy within 60s" >&2
        cat "${WORKDIR}/gateway.log" >&2
        exit 1
    fi
    sleep 1
done

osh() {
    "${OPENSHELL_BIN}" --gateway-endpoint "http://127.0.0.1:${GATEWAY_PORT}" "$@"
}

echo "Creating OpenShell sandbox ${SANDBOX_NAME}..."
osh sandbox create --name "${SANDBOX_NAME}"

echo "Uploading probe..."
osh sandbox upload "${SANDBOX_NAME}" "${PROBE_BIN}" /sandbox/sandbox-probe --no-git-ignore

osh sandbox exec "${SANDBOX_NAME}" -- chmod +x /sandbox/sandbox-probe

echo "Running sandbox-probe scan..."
osh sandbox exec "${SANDBOX_NAME}" -- /sandbox/sandbox-probe scan --fast --output_path /sandbox/r.json

osh sandbox exec "${SANDBOX_NAME}" -- cat /sandbox/r.json >"${WORKDIR}/report.json"

mkdir -p "${PROJECT_ROOT}/reports"
cp "${WORKDIR}/report.json" "${PROJECT_ROOT}/reports/sandbox-openshell.json"

printf '\n=== Report Generated ===\n'
jq '.' "${WORKDIR}/report.json"

printf '\n=== Verifying OpenShell Detection ===\n'
actual=$(jq -r '[.findings[] | select(.findingType == "sandbox_detection")] | .[0].value // "none"' "${WORKDIR}/report.json")
pass=$(jq 'any(.findings[]; .findingType == "sandbox_detection" and .value == "openshell")' "${WORKDIR}/report.json")

if [ "${pass}" = "true" ]; then
    echo "OpenShell detected: Test passed"
    exit 0
else
    echo "OpenShell not detected: Test failed (sandbox_detection reported '${actual}', expected 'openshell')"
    exit 1
fi
